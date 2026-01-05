// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

// This package has been refactored into multiple files for better organization:
// - model.go: Model struct and constructor
// - types.go: Type definitions, constants, and message types
// - update.go: Init() and Update() message handling
// - queries.go: Data fetching commands (fetch*, autoDetect*, etc.)
// - styles.go: UI styling
//
// The following files will be added to complete the refactoring:
// - handlers.go: Keyboard and mouse event handlers
// - render_*.go: Rendering functions split by feature
