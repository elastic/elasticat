// Copyright 2026 Elasticsearch B.V. and contributors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information set via ldflags at build time.
// Example: go build -ldflags "-X main.version=v1.0.0 -X main.commit=abc123"
var (
	version = "dev"     // Set via -ldflags "-X main.version=..."
	commit  = "unknown" // Set via -ldflags "-X main.commit=..."
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the version of elasticat",
	Long:  `Displays the version, commit, and build information for elasticat.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("elasticat %s\n", version)
		fmt.Printf("  commit:  %s\n", commit)
		fmt.Printf("  go:      %s\n", runtime.Version())
		fmt.Printf("  os/arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
