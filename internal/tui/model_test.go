package tui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sebavidal10/devclean/internal/plugins"
)

func TestTUIModelInitialization(t *testing.T) {
	m := NewModel(nil)
	if m.state != StateScanning {
		t.Errorf("expected initial state StateScanning, got %v", m.state)
	}

	view := m.View()
	if view == "" {
		t.Errorf("view should not be empty during scanning")
	}
}

func TestTUISelectionAndDrilldown(t *testing.T) {
	m := NewModel(nil)

	// Simulate scan finished message
	fakeReports := []plugins.PluginReport{
		{
			PluginID:   "xcode",
			Category:   "Apple Development",
			Title:      "Xcode DerivedData",
			SafetyNote: "Safe to delete",
			TotalBytes: 5000,
			Items: []plugins.ItemDetail{
				{ID: "item1", Description: "DerivedData/App1", SizeBytes: 3000, LastModDays: 10},
				{ID: "item2", Description: "DerivedData/App2", SizeBytes: 2000, LastModDays: 5},
			},
		},
	}

	updated, _ := m.Update(scanFinishedMsg{reports: fakeReports})
	m = updated.(Model)

	if m.state != StateSelection {
		t.Fatalf("expected state StateSelection, got %v", m.state)
	}

	if len(m.categories) != 1 {
		t.Fatalf("expected 1 category, got %d", len(m.categories))
	}

	if m.categories[0].SelectedBytes() != 5000 {
		t.Errorf("expected 5000 bytes selected, got %d", m.categories[0].SelectedBytes())
	}

	// Test drilldown transition
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'d'}})
	m = updated.(Model)

	if m.state != StateDrillDown {
		t.Fatalf("expected state StateDrillDown, got %v", m.state)
	}

	// Toggle first item in drilldown
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace})
	m = updated.(Model)

	if m.categories[0].SelectedItems["item1"] {
		t.Errorf("expected item1 to be unselected after space key")
	}

	// Return to selection with 'b'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'b'}})
	m = updated.(Model)

	if m.state != StateSelection {
		t.Fatalf("expected state StateSelection after b, got %v", m.state)
	}
}
