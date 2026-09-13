package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/sebavidal10/devclean/internal/plugins"
)

type failingCleanPlugin struct{}

func (*failingCleanPlugin) ID() string         { return "failing" }
func (*failingCleanPlugin) Category() string   { return "Test" }
func (*failingCleanPlugin) Name() string       { return "Failing plugin" }
func (*failingCleanPlugin) SafetyNote() string { return "Test only" }
func (*failingCleanPlugin) Detect() bool       { return true }
func (*failingCleanPlugin) Scan(context.Context) (plugins.PluginReport, error) {
	return plugins.PluginReport{}, nil
}
func (*failingCleanPlugin) Clean([]string) (int64, error) {
	return 128, errors.New("permission denied")
}

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

func TestTUICleanupRequiresConfirmationAndReportsPartialFailure(t *testing.T) {
	reg := plugins.NewRegistry()
	reg.Register(&failingCleanPlugin{})
	m := NewModel(reg)
	report := plugins.PluginReport{
		PluginID: "failing", Title: "Failing plugin", TotalBytes: 256,
		Items: []plugins.ItemDetail{{ID: "one", SizeBytes: 256}},
	}
	updated, _ := m.Update(scanFinishedMsg{reports: []plugins.PluginReport{report}})
	m = updated.(Model)

	updated, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if m.state != StateSelection || !m.confirmPending || cmd != nil {
		t.Fatalf("first c must request confirmation")
	}

	updated, cmd = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	m = updated.(Model)
	if m.state != StateCleaning || cmd == nil {
		t.Fatalf("second c must start cleanup")
	}

	msg := m.startCleanCmd()().(cleanFinishedMsg)
	updated, _ = m.Update(msg)
	m = updated.(Model)
	if m.state != StateSummary || m.freedBytes != 128 || m.err == nil {
		t.Fatalf("expected partial summary, got state=%v freed=%d err=%v", m.state, m.freedBytes, m.err)
	}
	if !strings.Contains(m.View(), "completada parcialmente") {
		t.Fatalf("partial failure is not visible in summary")
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
