// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirmY(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    string
		want  bool
		wantP bool
	}{
		{name: "lowercase_y", in: "y\n", want: true, wantP: true},
		{name: "uppercase_Y", in: "Y\n", want: true, wantP: true},
		{name: "whitespace_y", in: "  y  \n", want: true, wantP: true},
		{name: "empty", in: "\n", want: false, wantP: true},
		{name: "yes", in: "yes\n", want: false, wantP: true},
		{name: "other", in: "n\n", want: false, wantP: true},
		{name: "eof_no_newline", in: "y", want: true, wantP: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			var out bytes.Buffer
			ok, err := confirmY(strings.NewReader(tt.in), &out, "PROMPT> ")
			if err != nil {
				t.Fatalf("confirmY returned error: %v", err)
			}
			if ok != tt.want {
				t.Fatalf("confirmY(%q)=%v, want %v", tt.in, ok, tt.want)
			}
			if tt.wantP && !strings.HasPrefix(out.String(), "PROMPT> ") {
				t.Fatalf("expected prompt to be written, got %q", out.String())
			}
		})
	}
}

func TestIsLocalESURL(t *testing.T) {
	t.Parallel()

	tests := []struct {
		raw  string
		want bool
	}{
		{"http://localhost:9200", true},
		{"https://localhost:9200", true},
		{"localhost:9200", true},
		{"127.0.0.1:9200", true},
		{"http://127.0.0.1:9200", true},
		{"http://[::1]:9200", true},
		{"https://[::1]:9200", true},

		{"", false},
		{"http://example.com:9200", false},
		{"https://elastic.co", false},
		{"10.0.0.1:9200", false},
		{"http://10.0.0.1:9200", false},
		{"not a url", false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			if got := isLocalESURL(tt.raw); got != tt.want {
				t.Fatalf("isLocalESURL(%q)=%v, want %v", tt.raw, got, tt.want)
			}
		})
	}
}


