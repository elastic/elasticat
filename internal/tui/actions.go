// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

// Action represents a user action that can be triggered by one or more keys
type Action int

const (
	ActionNone Action = iota

	// Navigation - list/cursor movement
	ActionScrollUp
	ActionScrollDown
	ActionPageUp
	ActionPageDown
	ActionGoTop
	ActionGoBottom
	ActionPrevItem // left arrow, prev doc/metric
	ActionNextItem // right arrow, next doc/metric

	// Common actions
	ActionSelect        // enter - select/confirm
	ActionBack          // esc, backspace, q - go back/close
	ActionQuit          // q in main views - quit app
	ActionHelp          // ? - show help overlay
	ActionRefresh       // r - refresh data
	ActionSearch        // / - open search
	ActionCycleLookback // l - cycle time range
	ActionCycleSignal   // m - cycle signal type
	ActionPerspective   // p - perspective view/cycle
	ActionCopy          // y - copy to clipboard
	ActionJSON          // j - view as JSON (context-dependent)
	ActionKibana        // K - open in Kibana
	ActionSort          // s - toggle sort
	ActionFields        // f - field selector
	ActionQuery         // Q - show query
	ActionAutoRefresh   // a - toggle auto refresh
	ActionDashboard     // d - switch to dashboard view
	ActionDocuments     // d - switch to documents view (context)
	ActionToggle        // space - toggle selection
	ActionReset         // r - reset (in fields view)
	ActionSpans         // s - view spans (in trace detail)
	ActionNextDoc       // n - next document (in metric detail)
	ActionPrevDoc       // N - prev document (in metric detail)
	ActionChat          // c - open AI chat
	ActionCreds         // C - show credentials modal
)

// DefaultKeyBindings maps keys to their primary action.
// Uppercase keys are used for less common actions to avoid conflicts.
var DefaultKeyBindings = map[string]Action{
	// Navigation - includes vim keys (j/k) for list scrolling
	"up":     ActionScrollUp,
	"k":      ActionScrollUp,
	"down":   ActionScrollDown,
	"j":      ActionScrollDown,
	"pgup":   ActionPageUp,
	"pgdown": ActionPageDown,
	"home":   ActionGoTop,
	"g":      ActionGoTop,
	"end":    ActionGoBottom,
	"G":      ActionGoBottom,
	"left":   ActionPrevItem,
	"right":  ActionNextItem,

	// Common actions
	"enter":     ActionSelect,
	"esc":       ActionBack,
	"backspace": ActionBack,
	"q":         ActionQuit,
	"?":         ActionHelp,
	"r":         ActionRefresh,
	"/":         ActionSearch,
	"l":         ActionCycleLookback,
	"m":         ActionCycleSignal,
	"p":         ActionPerspective,
	"y":         ActionCopy,
	"K":         ActionKibana,
	"f":         ActionFields,
	"Q":         ActionQuery,
	"a":         ActionAutoRefresh,
	"space":     ActionToggle,
	"s":         ActionSort,

	// Uppercase keys for specific view actions
	"J": ActionJSON,    // JSON view in detail views
	"S": ActionSpans,   // Spans view in trace detail
	"n": ActionNextDoc, // Next document in metric detail
	"N": ActionPrevDoc, // Prev document in metric detail
	"c": ActionChat,    // Open AI chat
	"C": ActionCreds,   // Show credentials modal

	// Context-dependent keys (handled specially in some views)
	// "d" - dashboard/documents toggle (not in default map)
}

// GetAction returns the action for a key from the default bindings.
// Returns ActionNone if the key is not bound.
func GetAction(key string) Action {
	if action, ok := DefaultKeyBindings[key]; ok {
		return action
	}
	return ActionNone
}

// IsNavAction returns true if the action is a navigation action
func IsNavAction(action Action) bool {
	return action >= ActionScrollUp && action <= ActionNextItem
}

// IsListNavAction returns true if the action is for list navigation (up/down/page/home/end)
func IsListNavAction(action Action) bool {
	return action >= ActionScrollUp && action <= ActionGoBottom
}

// ActionInfo provides display information for an action
type ActionInfo struct {
	DisplayKeys []string // Keys to show in help bar (e.g., ["↑", "↓"])
	Label       string   // Label for the action (e.g., "scroll")
}

// ActionDisplay maps actions to their display information
var ActionDisplay = map[Action]ActionInfo{
	ActionScrollUp:      {DisplayKeys: []string{"↑"}, Label: "up"},
	ActionScrollDown:    {DisplayKeys: []string{"↓"}, Label: "down"},
	ActionPageUp:        {DisplayKeys: []string{"pgup"}, Label: "page up"},
	ActionPageDown:      {DisplayKeys: []string{"pgdown"}, Label: "page down"},
	ActionGoTop:         {DisplayKeys: []string{"g"}, Label: "top"},
	ActionGoBottom:      {DisplayKeys: []string{"G"}, Label: "bottom"},
	ActionPrevItem:      {DisplayKeys: []string{"←"}, Label: "prev"},
	ActionNextItem:      {DisplayKeys: []string{"→"}, Label: "next"},
	ActionSelect:        {DisplayKeys: []string{"enter"}, Label: "select"},
	ActionBack:          {DisplayKeys: []string{"esc"}, Label: "back"},
	ActionQuit:          {DisplayKeys: []string{"q"}, Label: "quit"},
	ActionHelp:          {DisplayKeys: []string{"?"}, Label: "help"},
	ActionRefresh:       {DisplayKeys: []string{"r"}, Label: "refresh"},
	ActionSearch:        {DisplayKeys: []string{"/"}, Label: "search"},
	ActionCycleLookback: {DisplayKeys: []string{"l"}, Label: "lookback"},
	ActionCycleSignal:   {DisplayKeys: []string{"m"}, Label: "signal"},
	ActionPerspective:   {DisplayKeys: []string{"p"}, Label: "perspective"},
	ActionCopy:          {DisplayKeys: []string{"y"}, Label: "copy"},
	ActionJSON:          {DisplayKeys: []string{"J"}, Label: "JSON"},
	ActionKibana:        {DisplayKeys: []string{"K"}, Label: "kibana"},
	ActionSort:          {DisplayKeys: []string{"s"}, Label: "sort"},
	ActionFields:        {DisplayKeys: []string{"f"}, Label: "fields"},
	ActionQuery:         {DisplayKeys: []string{"Q"}, Label: "query"},
	ActionAutoRefresh:   {DisplayKeys: []string{"a"}, Label: "auto refresh"},
	ActionToggle:        {DisplayKeys: []string{"space"}, Label: "toggle"},
	ActionReset:         {DisplayKeys: []string{"r"}, Label: "reset"},
	ActionSpans:         {DisplayKeys: []string{"S"}, Label: "spans"},
	ActionNextDoc:       {DisplayKeys: []string{"n"}, Label: "next doc"},
	ActionPrevDoc:       {DisplayKeys: []string{"N"}, Label: "prev doc"},
	ActionChat:          {DisplayKeys: []string{"c"}, Label: "chat"},
	ActionCreds:         {DisplayKeys: []string{"C"}, Label: "creds"},
}

// ScrollDisplayKeys returns the combined display for scroll up/down
var ScrollDisplayKeys = []string{"↑", "↓"}

// PrevNextDisplayKeys returns the combined display for prev/next item
var PrevNextDisplayKeys = []string{"←", "→"}

// ActionBinding creates a KeyBinding from an action
func ActionBinding(action Action, kind KeyKind, group string) KeyBinding {
	info := ActionDisplay[action]
	return KeyBinding{
		Keys:  info.DisplayKeys,
		Label: info.Label,
		Kind:  kind,
		Group: group,
	}
}

// ActionBindingWithLabel creates a KeyBinding from an action with a custom label
func ActionBindingWithLabel(action Action, label string, kind KeyKind, group string) KeyBinding {
	info := ActionDisplay[action]
	return KeyBinding{
		Keys:  info.DisplayKeys,
		Label: label,
		Kind:  kind,
		Group: group,
	}
}

// CombinedBinding creates a KeyBinding from multiple keys with a custom label
func CombinedBinding(keys []string, label string, kind KeyKind, group string) KeyBinding {
	return KeyBinding{
		Keys:  keys,
		Label: label,
		Kind:  kind,
		Group: group,
	}
}

// ScrollBinding returns a standard scroll up/down binding
func ScrollBinding(kind KeyKind) KeyBinding {
	return CombinedBinding(ScrollDisplayKeys, "scroll", kind, "Navigation")
}

// PrevNextBinding returns a standard prev/next item binding
func PrevNextBinding(label string, kind KeyKind) KeyBinding {
	return CombinedBinding(PrevNextDisplayKeys, label, kind, "Navigation")
}
