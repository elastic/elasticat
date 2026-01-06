// Copyright 2024-2025 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

//go:build tools

package tools

import (
	// Tool dependencies - these are kept in go.mod but not compiled into the binary
	_ "go.elastic.co/go-licence-detector"
)
