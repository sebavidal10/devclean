package plugins

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type XcodePlugin struct{}

func NewXcodePlugin() *XcodePlugin {
	return &XcodePlugin{}
}

func (x *XcodePlugin) ID() string {
	return "xcode"
}

func (x *XcodePlugin) Category() string {
	return "Apple Development"
}

func (x *XcodePlugin) Name() string {
	return "Xcode & Simulators"
}

func (x *XcodePlugin) SafetyNote() string {
	return "DerivedData and Simulator caches will be safely rebuilt by Xcode on next compile. Project sources are never touched."
}

func (x *XcodePlugin) Detect() bool {
	// Check standard Xcode locations or developer directory
	home, err := os.UserHomeDir()
	if err == nil {
		devDir := filepath.Join(home, "Library", "Developer")
		if _, err := os.Stat(devDir); err == nil {
			return true
		}
	}
	if _, err := os.Stat("/Applications/Xcode.app"); err == nil {
		return true
	}
	if _, err := exec.LookPath("xcode-select"); err == nil {
		cmd := exec.Command("xcode-select", "-p")
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	return false
}

func (x *XcodePlugin) Scan(ctx context.Context) (PluginReport, error) {
	report := PluginReport{
		PluginID:   x.ID(),
		Category:   x.Category(),
		Title:      x.Name(),
		SafetyNote: x.SafetyNote(),
		Items:      make([]ItemDetail, 0),
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return report, fmt.Errorf("unable to resolve user home: %w", err)
	}

	targets := []struct {
		rootPath    string
		descPrefix  string
		categoryTag string
	}{
		{
			rootPath:    filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData"),
			descPrefix:  "Xcode DerivedData: ",
			categoryTag: "derived-data",
		},
		{
			rootPath:    filepath.Join(home, "Library", "Developer", "CoreSimulator", "Caches"),
			descPrefix:  "Simulator Cache: ",
			categoryTag: "simulator-cache",
		},
	}

	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		entries, err := os.ReadDir(target.rootPath)
		if err != nil {
			// Directory might not exist yet, continue gracefully
			continue
		}

		for _, entry := range entries {
			if err := ctx.Err(); err != nil {
				return report, err
			}
			fullPath := filepath.Join(target.rootPath, entry.Name())
			// Safety validation before listing
			if err := IsSafePath(fullPath); err != nil {
				continue
			}

			size, err := DirSizeContext(ctx, fullPath)
			if err != nil || size == 0 {
				continue
			}

			age := PathAgeDays(fullPath)
			itemID := fmt.Sprintf("%s:%s", target.categoryTag, entry.Name())

			report.Items = append(report.Items, ItemDetail{
				ID:          itemID,
				Path:        fullPath,
				Description: target.descPrefix + entry.Name(),
				SizeBytes:   size,
				LastModDays: age,
			})
			report.TotalBytes += size
		}
	}

	return report, nil
}

func (x *XcodePlugin) Clean(itemIDs []string) (int64, error) {
	report, err := x.Scan(context.Background())
	if err != nil {
		return 0, err
	}

	targetsToClean := make(map[string]ItemDetail)
	for _, it := range report.Items {
		targetsToClean[it.ID] = it
	}

	var freedBytes int64
	for _, id := range itemIDs {
		item, exists := targetsToClean[id]
		if !exists {
			continue
		}

		home, _ := os.UserHomeDir()
		allowedRoots := []string{
			filepath.Join(home, "Library", "Developer", "Xcode", "DerivedData"),
			filepath.Join(home, "Library", "Developer", "CoreSimulator", "Caches"),
		}
		if err := SafeRemoveAll(item.Path, allowedRoots...); err != nil {
			return freedBytes, fmt.Errorf("failed to clean %s: %w", item.Path, err)
		}
		freedBytes += FreedBytesAfterCleanup(item.Path, item.SizeBytes)
	}

	return freedBytes, nil
}
