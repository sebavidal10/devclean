package plugins

import (
	"context"
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

func (m *mockPlugin) ID() string         { return m.id }
func (m *mockPlugin) Category() string   { return m.category }
func (m *mockPlugin) Name() string       { return m.name }
func (m *mockPlugin) SafetyNote() string { return "Mock safety note" }
func (m *mockPlugin) Detect() bool       { return m.detected }
func (m *mockPlugin) Scan() (PluginReport, error) {
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

func (s *slowMockPlugin) Scan() (PluginReport, error) {
	time.Sleep(100 * time.Millisecond)
	return PluginReport{}, nil
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
