package tui

import (
	"context"
	"testing"
	"time"
)

func TestStartRequestCancelsPrevious(t *testing.T) {
	m := Model{
		ctx:     context.Background(),
		cancels: make(map[requestKind]requestState),
	}

	ctx1, done1 := m.startRequest(requestLogs, time.Hour)
	t.Cleanup(done1)

	select {
	case <-ctx1.Done():
		t.Fatalf("first request should not be canceled immediately")
	default:
	}

	ctx2, done2 := m.startRequest(requestLogs, time.Hour)
	t.Cleanup(done2)

	select {
	case <-ctx1.Done():
		// expected: superseded request is canceled
	default:
		t.Fatalf("expected first request to be canceled after supersede")
	}

	if ctx2.Err() != nil {
		t.Fatalf("second context should be active, got err=%v", ctx2.Err())
	}

	if len(m.cancels) != 1 {
		t.Fatalf("expected one cancel entry, got %d", len(m.cancels))
	}
}

func TestStartRequestDoneDoesNotClearNewer(t *testing.T) {
	m := Model{
		ctx:     context.Background(),
		cancels: make(map[requestKind]requestState),
	}

	_, done1 := m.startRequest(requestLogs, time.Hour)
	ctx2, done2 := m.startRequest(requestLogs, time.Hour)
	t.Cleanup(done2)

	// Calling done1 (old request) should not remove the current cancel entry.
	done1()

	if _, ok := m.cancels[requestLogs]; !ok {
		t.Fatalf("newer request cancel should remain after old done")
	}
	if ctx2.Err() != nil {
		t.Fatalf("current context should still be active")
	}
}

func TestHandleLogsMsgIgnoresCanceled(t *testing.T) {
	m := Model{mode: viewLogs, loading: true}
	m2, _ := m.handleLogsMsg(logsMsg{err: context.Canceled})

	if m2.mode != viewLogs {
		t.Fatalf("mode changed unexpectedly: %v", m2.mode)
	}
	if m2.err != nil {
		t.Fatalf("err should remain nil on cancellation, got %v", m2.err)
	}
	if m2.loading {
		t.Fatalf("loading should be false after handling logsMsg")
	}
}
