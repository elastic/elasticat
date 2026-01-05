// Copyright 2026 Elasticsearch B.V.
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/elastic/elasticat/internal/es"
)

func (m Model) handleFieldsKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// If in search mode, handle text input
	if m.fieldsSearchMode {
		switch msg.String() {
		case "esc":
			m.fieldsSearchMode = false
			m.fieldsSearch = ""
			return m, nil
		case "enter":
			m.fieldsSearchMode = false
			return m, nil
		case "backspace":
			if len(m.fieldsSearch) > 0 {
				m.fieldsSearch = m.fieldsSearch[:len(m.fieldsSearch)-1]
			}
			return m, nil
		default:
			// Add character to search
			if len(msg.String()) == 1 {
				m.fieldsSearch += msg.String()
			}
			return m, nil
		}
	}

	// Get the sorted field list for navigation
	sortedFields := m.getSortedFieldList()

	switch msg.String() {
	case "esc", "q":
		m.mode = viewLogs
		return m, nil
	case "up", "k":
		if m.fieldsCursor > 0 {
			m.fieldsCursor--
		}
	case "down", "j":
		if m.fieldsCursor < len(sortedFields)-1 {
			m.fieldsCursor++
		}
	case "home", "g":
		m.fieldsCursor = 0
	case "end", "G":
		if len(sortedFields) > 0 {
			m.fieldsCursor = len(sortedFields) - 1
		}
	case "pgup":
		m.fieldsCursor -= 10
		if m.fieldsCursor < 0 {
			m.fieldsCursor = 0
		}
	case "pgdown":
		m.fieldsCursor += 10
		if m.fieldsCursor >= len(sortedFields) {
			m.fieldsCursor = len(sortedFields) - 1
		}
	case " ", "enter":
		// Toggle field selection
		if m.fieldsCursor < len(sortedFields) {
			fieldName := sortedFields[m.fieldsCursor].Name
			m.toggleField(fieldName)
		}
	case "/":
		m.fieldsSearchMode = true
		m.fieldsSearch = ""
	case "r":
		// Reset to defaults for current signal type
		m.displayFields = DefaultFields(m.signalType)
	}

	return m, nil
}

// toggleField toggles a field's selection state
func (m *Model) toggleField(fieldName string) {
	// Check if it's already in displayFields
	for i, f := range m.displayFields {
		if f.Name == fieldName {
			// Remove it
			m.displayFields = append(m.displayFields[:i], m.displayFields[i+1:]...)
			return
		}
	}

	// Add it as a new field
	label := fieldName
	// Use last part of field name as label
	if idx := strings.LastIndex(fieldName, "."); idx >= 0 {
		label = fieldName[idx+1:]
	}
	label = strings.ToUpper(label)
	if len(label) > 12 {
		label = label[:12]
	}

	// Determine if this field should be searchable
	// Skip timestamp-like and numeric fields
	var searchFields []string
	if !strings.Contains(fieldName, "timestamp") && !strings.Contains(fieldName, "time") {
		searchFields = []string{} // Empty slice means use Name as search field
	}

	m.displayFields = append(m.displayFields, DisplayField{
		Name:         fieldName,
		Label:        label,
		Width:        15, // Default width for custom fields
		Selected:     true,
		SearchFields: searchFields,
	})
}

// getSortedFieldList returns available fields sorted with selected/default fields first
func (m Model) getSortedFieldList() []es.FieldInfo {
	// Create a map of selected field names
	selectedNames := make(map[string]bool)
	for _, f := range m.displayFields {
		selectedNames[f.Name] = true
	}

	// Create a map of available fields for quick lookup
	availableByName := make(map[string]es.FieldInfo)
	for _, f := range m.availableFields {
		availableByName[f.Name] = f
	}

	// Filter available fields by search if active
	var filtered []es.FieldInfo
	for _, f := range m.availableFields {
		if m.fieldsSearch != "" {
			if !strings.Contains(strings.ToLower(f.Name), strings.ToLower(m.fieldsSearch)) {
				continue
			}
		}
		filtered = append(filtered, f)
	}

	// Also filter display fields by search
	var filteredDisplayFields []DisplayField
	for _, df := range m.displayFields {
		if m.fieldsSearch != "" {
			if !strings.Contains(strings.ToLower(df.Name), strings.ToLower(m.fieldsSearch)) {
				continue
			}
		}
		filteredDisplayFields = append(filteredDisplayFields, df)
	}

	// Sort: selected fields first (in display order), then others by doc count
	result := make([]es.FieldInfo, 0, len(filtered)+len(filteredDisplayFields))

	// First, add selected fields in their current display order
	// Try to find matching FieldInfo to get DocCount, otherwise create one
	for _, df := range filteredDisplayFields {
		if f, ok := availableByName[df.Name]; ok {
			// Found exact match - use it with its DocCount
			result = append(result, f)
		} else {
			// No exact match (virtual field like _resource, or different naming)
			// Try to find a related field for the count
			var docCount int64
			for _, searchField := range df.SearchFields {
				if f, ok := availableByName[searchField]; ok {
					if f.DocCount > docCount {
						docCount = f.DocCount
					}
				}
			}
			// Create a FieldInfo for this display field
			result = append(result, es.FieldInfo{
				Name:         df.Name,
				Type:         "display", // Mark as display-only field
				Searchable:   len(df.SearchFields) > 0 || len(df.GetSearchFields()) > 0,
				Aggregatable: false,
				DocCount:     docCount,
			})
		}
	}

	// Then add non-selected fields sorted by doc count (descending)
	var nonSelected []es.FieldInfo
	for _, f := range filtered {
		if !selectedNames[f.Name] {
			nonSelected = append(nonSelected, f)
		}
	}
	// Sort non-selected by DocCount descending (most popular first)
	for i := 0; i < len(nonSelected); i++ {
		for j := i + 1; j < len(nonSelected); j++ {
			if nonSelected[i].DocCount < nonSelected[j].DocCount {
				nonSelected[i], nonSelected[j] = nonSelected[j], nonSelected[i]
			}
		}
	}
	result = append(result, nonSelected...)

	return result
}

