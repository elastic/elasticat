// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"
)

// keyHint renders a consistent key hint like: "[k1/k2] label".
// This should be the only way UI code formats key hints with brackets.
func keyHint(keys []string, label string) string {
	if len(keys) == 0 {
		return label
	}
	return "[" + strings.Join(keys, "/") + "] " + label
}

// actionHint renders a key hint for an action using ActionDisplay.
func actionHint(action Action) string {
	info := ActionDisplay[action]
	return keyHint(info.DisplayKeys, info.Label)
}

// actionsHint renders a key hint for multiple actions (e.g., scroll up/down) under one label.
func actionsHint(label string, actions ...Action) string {
	var keys []string
	for _, a := range actions {
		info, ok := ActionDisplay[a]
		if !ok {
			continue
		}
		keys = append(keys, info.DisplayKeys...)
	}
	return keyHint(keys, label)
}

// keysHint renders a key hint from explicit keys (use sparingly for context-only keys like "0-4" or "d").
func keysHint(label string, keys ...string) string {
	return keyHint(keys, label)
}


