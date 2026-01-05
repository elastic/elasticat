// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/nxadm/tail"
)

// LogHandler is called for each parsed log line
type LogHandler func(log ParsedLog)

// Watcher watches multiple log files and calls handlers for each line
type Watcher struct {
	files     []string
	service   string
	tailLines int
	follow    bool
	noColor   bool
	oneshot   bool
	handlers  []LogHandler
	tails     []*tail.Tail
	mu        sync.Mutex
	ctx       context.Context
	cancel    context.CancelFunc
}

// Config holds watcher configuration
type Config struct {
	Files     []string
	Service   string // Override service name
	TailLines int    // Number of lines to show initially (0 = all lines in oneshot mode)
	Follow    bool   // Keep watching for new lines
	NoColor   bool   // Disable colored output
	Oneshot   bool   // Read all lines and exit (don't follow)
}

// New creates a new Watcher
func New(cfg Config) (*Watcher, error) {
	// Expand globs
	var files []string
	for _, pattern := range cfg.Files {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid pattern %q: %w", pattern, err)
		}
		if len(matches) == 0 {
			// Check if it's a literal file that doesn't exist yet
			if _, err := os.Stat(pattern); os.IsNotExist(err) {
				fmt.Fprintf(os.Stderr, "Warning: file %q does not exist (will watch for creation)\n", pattern)
			}
			files = append(files, pattern)
		} else {
			files = append(files, matches...)
		}
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("no files to watch")
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &Watcher{
		files:     files,
		service:   cfg.Service,
		tailLines: cfg.TailLines,
		follow:    cfg.Follow,
		noColor:   cfg.NoColor,
		oneshot:   cfg.Oneshot,
		handlers:  make([]LogHandler, 0),
		tails:     make([]*tail.Tail, 0),
		ctx:       ctx,
		cancel:    cancel,
	}, nil
}

// AddHandler adds a log handler
func (w *Watcher) AddHandler(h LogHandler) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers = append(w.handlers, h)
}

// Start begins watching all files
func (w *Watcher) Start() error {
	var wg sync.WaitGroup

	for _, file := range w.files {
		wg.Add(1)
		go func(f string) {
			defer wg.Done()
			if err := w.watchFile(f); err != nil {
				fmt.Fprintf(os.Stderr, "Error watching %s: %v\n", f, err)
			}
		}(file)
	}

	// Wait for context cancellation
	<-w.ctx.Done()

	// Stop all tails
	w.mu.Lock()
	for _, t := range w.tails {
		t.Stop()
	}
	w.mu.Unlock()

	wg.Wait()
	return nil
}

// Stop stops watching all files
func (w *Watcher) Stop() {
	w.cancel()
}

// FileCount returns the number of files being watched
func (w *Watcher) FileCount() int {
	return len(w.files)
}

// Files returns the list of files being watched (after glob expansion)
func (w *Watcher) Files() []string {
	return w.files
}

// ReadAll reads all lines from all files and calls handlers for each
// This is used for oneshot mode - reads everything and returns
func (w *Watcher) ReadAll() (int, error) {
	totalLines := 0

	for _, filename := range w.files {
		service := w.service
		if service == "" {
			service = ServiceFromFilename(filename)
		}

		file, err := os.Open(filename)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Warning: could not open %s: %v\n", filename, err)
			continue
		}

		// Read all lines
		buf := make([]byte, 64*1024)
		var partial string
		var lines []string

		for {
			n, err := file.Read(buf)
			if n > 0 {
				chunk := partial + string(buf[:n])
				parts := splitLines(chunk)
				if len(parts) > 0 {
					partial = parts[len(parts)-1]
					lines = append(lines, parts[:len(parts)-1]...)
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				file.Close()
				return totalLines, fmt.Errorf("error reading %s: %w", filename, err)
			}
		}
		file.Close()

		// Don't forget remaining partial line
		if partial != "" {
			lines = append(lines, partial)
		}

		// Process all lines
		for _, line := range lines {
			if line == "" {
				continue
			}
			parsed := ParseLine(line, filename, service)
			w.callHandlers(parsed)
			totalLines++
		}
	}

	return totalLines, nil
}

func (w *Watcher) watchFile(filename string) error {
	// Determine service name
	service := w.service
	if service == "" {
		service = ServiceFromFilename(filename)
	}

	// Read and display the last N lines if requested
	if w.tailLines > 0 {
		if err := w.showLastLines(filename, service); err != nil {
			// Not fatal - file might not exist yet
			fmt.Fprintf(os.Stderr, "Warning: could not read initial lines from %s: %v\n", filename, err)
		}
	}

	// Start tailing from end of file
	cfg := tail.Config{
		Follow:    w.follow,
		ReOpen:    true,  // Handle file rotation
		MustExist: false, // Allow watching files that don't exist yet
		Poll:      true,  // Use polling (more reliable across filesystems)
		Location:  &tail.SeekInfo{Offset: 0, Whence: io.SeekEnd},
	}

	t, err := tail.TailFile(filename, cfg)
	if err != nil {
		return fmt.Errorf("failed to tail %s: %w", filename, err)
	}

	w.mu.Lock()
	w.tails = append(w.tails, t)
	w.mu.Unlock()

	// Process new lines as they arrive
	for {
		select {
		case <-w.ctx.Done():
			return nil
		case line, ok := <-t.Lines:
			if !ok {
				return nil
			}
			if line.Err != nil {
				fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", filename, line.Err)
				continue
			}

			// Parse and handle the line
			parsed := ParseLine(line.Text, filename, service)
			w.callHandlers(parsed)
		}
	}
}

// showLastLines reads and displays the last N lines from a file
func (w *Watcher) showLastLines(filename, service string) error {
	file, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	// Read all lines (simple approach - could be optimized for large files)
	var lines []string
	buf := make([]byte, 64*1024) // 64KB buffer
	var partial string

	for {
		n, err := file.Read(buf)
		if n > 0 {
			chunk := partial + string(buf[:n])
			parts := splitLines(chunk)
			if len(parts) > 0 {
				// Last part might be incomplete
				partial = parts[len(parts)-1]
				lines = append(lines, parts[:len(parts)-1]...)
			}
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}
	// Don't forget any remaining partial line
	if partial != "" {
		lines = append(lines, partial)
	}

	// Get last N lines
	start := 0
	if len(lines) > w.tailLines {
		start = len(lines) - w.tailLines
	}

	// Display them
	for _, line := range lines[start:] {
		if line == "" {
			continue
		}
		parsed := ParseLine(line, filename, service)
		w.callHandlers(parsed)
	}

	return nil
}

// splitLines splits a string into lines, preserving the last potentially incomplete line
func splitLines(s string) []string {
	var lines []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			line := s[start:i]
			if len(line) > 0 && line[len(line)-1] == '\r' {
				line = line[:len(line)-1]
			}
			lines = append(lines, line)
			start = i + 1
		}
	}
	// Include any trailing content (potentially incomplete line)
	if start < len(s) {
		lines = append(lines, s[start:])
	}
	return lines
}

func (w *Watcher) callHandlers(log ParsedLog) {
	w.mu.Lock()
	handlers := make([]LogHandler, len(w.handlers))
	copy(handlers, w.handlers)
	w.mu.Unlock()

	for _, h := range handlers {
		h(log)
	}
}

// FormatLog formats a parsed log for terminal output
func FormatLog(log ParsedLog, noColor bool, showFilename bool) string {
	var prefix string
	if showFilename {
		prefix = fmt.Sprintf("[%-15s] ", truncate(filepath.Base(log.Source), 15))
	}

	ts := log.Timestamp.Format("15:04:05.000")
	level := fmt.Sprintf("%-5s", log.Level)

	if noColor {
		return fmt.Sprintf("%s%s %s %s", prefix, ts, level, log.Message)
	}

	return fmt.Sprintf("%s%s %s%s%s %s",
		prefix,
		ts,
		log.Level.Color(),
		level,
		ColorReset(),
		log.Message,
	)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}
