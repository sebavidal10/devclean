package plugins

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNodeScanUsesConfiguredWorkspaceAndAge(t *testing.T) {
	workspace := t.TempDir()
	modules := filepath.Join(workspace, "project", "node_modules")
	if err := os.MkdirAll(modules, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(modules, "package.js"), []byte("cache"), 0600); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-61 * 24 * time.Hour)
	if err := os.Chtimes(modules, old, old); err != nil {
		t.Fatal(err)
	}

	plugin := &NodePlugin{workspaceDir: workspace, npmCacheDir: filepath.Join(workspace, "missing-npm-cache"), inactiveDays: 60}
	report, err := plugin.Scan(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Items) != 1 || report.Items[0].Path != modules {
		t.Fatalf("unexpected items: %+v", report.Items)
	}
}

func TestNodeScanHonorsCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	plugin := &NodePlugin{workspaceDir: t.TempDir(), npmCacheDir: filepath.Join(t.TempDir(), "missing"), inactiveDays: 30}
	_, err := plugin.Scan(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context cancellation, got %v", err)
	}
}
