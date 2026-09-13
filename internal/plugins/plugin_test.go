package plugins

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type mockPlugin struct {
	id       string
	category string
	name     string
	detected bool
	report   PluginReport
	err      error
}

func TestRegistryScanAllReturnsPartialResultsAndErrorsInOrder(t *testing.T) {
	reg := NewRegistry()
	reg.Register(&mockPlugin{id: "z", name: "Z", detected: true, report: PluginReport{
		PluginID: "z", Items: []ItemDetail{{ID: "b"}, {ID: "a"}},
	}})
	reg.Register(&mockPlugin{id: "broken", name: "Broken", detected: true, err: errors.New("boom")})
	reg.Register(&mockPlugin{id: "a", name: "A", detected: true, report: PluginReport{PluginID: "a"}})

	reports, err := reg.ScanAll(context.Background())
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected joined scan error, got %v", err)
	}
	if len(reports) != 2 || reports[0].PluginID != "a" || reports[1].PluginID != "z" {
		t.Fatalf("reports are not sorted: %+v", reports)
	}
	if reports[1].Items[0].ID != "a" {
		t.Fatalf("items are not sorted: %+v", reports[1].Items)
	}
}

func (m *mockPlugin) ID() string         { return m.id }
func (m *mockPlugin) Category() string   { return m.category }
func (m *mockPlugin) Name() string       { return m.name }
func (m *mockPlugin) SafetyNote() string { return "Mock safety note" }
func (m *mockPlugin) Detect() bool       { return m.detected }
func (m *mockPlugin) Scan(context.Context) (PluginReport, error) {
	return m.report, m.err
}
func (m *mockPlugin) Clean(itemIDs []string) (int64, error) {
	return m.report.TotalBytes, nil
}

func TestRegistryScanAll(t *testing.T) {
	reg := NewRegistry()

	p1 := &mockPlugin{
		id:       "p1",
		category: "Mock",
		name:     "Plugin 1",
		detected: true,
		report: PluginReport{
			PluginID:   "p1",
			Category:   "Mock",
			Title:      "Plugin 1",
			TotalBytes: 1024,
			Items: []ItemDetail{
				{ID: "item1", SizeBytes: 1024},
			},
		},
	}

	p2 := &mockPlugin{
		id:       "p2",
		category: "Mock",
		name:     "Plugin 2",
		detected: false, // Not detected
	}

	p3 := &mockPlugin{
		id:       "p3",
		category: "Mock",
		name:     "Plugin 3",
		detected: true,
		report: PluginReport{
			PluginID:   "p3",
			Category:   "Mock",
			Title:      "Plugin 3",
			TotalBytes: 2048,
			Items: []ItemDetail{
				{ID: "item3", SizeBytes: 2048},
			},
		},
	}

	reg.Register(p1)
	reg.Register(p2)
	reg.Register(p3)

	reports, err := reg.ScanAll(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(reports) != 2 {
		t.Fatalf("expected 2 reports from detected plugins, got %d", len(reports))
	}

	var total int64
	for _, rep := range reports {
		total += rep.TotalBytes
	}

	if total != 3072 {
		t.Errorf("expected total bytes 3072, got %d", total)
	}
}

type slowMockPlugin struct {
	mockPlugin
}

func (s *slowMockPlugin) Scan(ctx context.Context) (PluginReport, error) {
	select {
	case <-time.After(100 * time.Millisecond):
		return PluginReport{}, nil
	case <-ctx.Done():
		return PluginReport{}, ctx.Err()
	}
}

func TestRegistryScanAllTimeout(t *testing.T) {
	reg := NewRegistry()

	slow := &slowMockPlugin{
		mockPlugin: mockPlugin{
			id:       "slow",
			category: "Slow",
			name:     "Slow Plugin",
			detected: true,
		},
	}
	reg.Register(slow)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Millisecond)
	defer cancel()

	_, err := reg.ScanAll(ctx)
	if err == nil {
		t.Errorf("expected context cancellation error, got nil")
	}
}
