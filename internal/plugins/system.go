package plugins

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

type SystemPlugin struct{}

func NewSystemPlugin() *SystemPlugin {
	return &SystemPlugin{}
}

func (s *SystemPlugin) ID() string {
	return "system-logs"
}

func (s *SystemPlugin) Category() string {
	return "macOS System & Diagnostic Caches"
}

func (s *SystemPlugin) Name() string {
	return "System & Application Logs"
}

func (s *SystemPlugin) SafetyNote() string {
	return "Safely clears diagnostic and application logs in ~/Library/Logs. Active applications will recreate log files as needed."
}

func (s *SystemPlugin) Detect() bool {
	return runtime.GOOS == "darwin"
}

func (s *SystemPlugin) Scan(ctx context.Context) (PluginReport, error) {
	report := PluginReport{
		PluginID:   s.ID(),
		Category:   s.Category(),
		Title:      s.Name(),
		SafetyNote: s.SafetyNote(),
		Items:      make([]ItemDetail, 0),
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return report, fmt.Errorf("unable to resolve user home: %w", err)
	}

	logsDir := filepath.Join(home, "Library", "Logs")
	entries, err := os.ReadDir(logsDir)
	if err != nil {
		return report, nil
	}

	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		fullPath := filepath.Join(logsDir, entry.Name())
		if err := IsSafePath(fullPath); err != nil {
			continue
		}

		var size int64
		if entry.IsDir() {
			size, err = DirSizeContext(ctx, fullPath)
			if err != nil {
				return report, err
			}
		} else {
			info, err := entry.Info()
			if err == nil {
				size = info.Size()
			}
		}

		// Filter negligible items (< 1MB) to avoid cluttering report
		if size < 1024*1024 {
			continue
		}

		age := PathAgeDays(fullPath)
		report.Items = append(report.Items, ItemDetail{
			ID:          "log:" + entry.Name(),
			Path:        fullPath,
			Description: fmt.Sprintf("App Log: %s", entry.Name()),
			SizeBytes:   size,
			LastModDays: age,
		})
		report.TotalBytes += size
	}

	return report, nil
}

func (s *SystemPlugin) Clean(itemIDs []string) (int64, error) {
	report, err := s.Scan(context.Background())
	if err != nil {
		return 0, err
	}

	itemMap := make(map[string]ItemDetail)
	for _, it := range report.Items {
		itemMap[it.ID] = it
	}

	var freedBytes int64
	for _, id := range itemIDs {
		item, exists := itemMap[id]
		if !exists {
			continue
		}

		home, _ := os.UserHomeDir()
		if err := SafeRemoveAll(item.Path, filepath.Join(home, "Library", "Logs")); err != nil {
			return freedBytes, fmt.Errorf("failed to clean log path %s: %w", item.Path, err)
		}
		freedBytes += FreedBytesAfterCleanup(item.Path, item.SizeBytes)
	}

	return freedBytes, nil
}
