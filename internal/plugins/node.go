package plugins

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type NodePlugin struct {
	workspaceDir string
}

func NewNodePlugin() *NodePlugin {
	home, _ := os.UserHomeDir()
	ws := filepath.Join(home, "Workspace")
	return &NodePlugin{
		workspaceDir: ws,
	}
}

func (n *NodePlugin) ID() string {
	return "node"
}

func (n *NodePlugin) Category() string {
	return "JavaScript & Node.js"
}

func (n *NodePlugin) Name() string {
	return "Node.js & NPM"
}

func (n *NodePlugin) SafetyNote() string {
	return "Removes inactive node_modules (>30 days untouched) and global npm cache. Reinstallable with 'npm install' or 'pnpm install'."
}

func (n *NodePlugin) Detect() bool {
	home, err := os.UserHomeDir()
	if err == nil {
		npmDir := filepath.Join(home, ".npm")
		if _, err := os.Stat(npmDir); err == nil {
			return true
		}
	}
	if _, err := exec.LookPath("npm"); err == nil {
		return true
	}
	if _, err := exec.LookPath("node"); err == nil {
		return true
	}
	return false
}

func (n *NodePlugin) Scan() (PluginReport, error) {
	report := PluginReport{
		PluginID:   n.ID(),
		Category:   n.Category(),
		Title:      n.Name(),
		SafetyNote: n.SafetyNote(),
		Items:      make([]ItemDetail, 0),
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return report, fmt.Errorf("unable to resolve user home: %w", err)
	}

	// 1. Scan global npm cache (~/.npm)
	npmCache := filepath.Join(home, ".npm")
	if info, err := os.Stat(npmCache); err == nil && info.IsDir() {
		if err := IsSafePath(npmCache); err == nil {
			size, _ := DirSize(npmCache)
			if size > 0 {
				age := PathAgeDays(npmCache)
				report.Items = append(report.Items, ItemDetail{
					ID:          "npm-cache",
					Path:        npmCache,
					Description: "Global NPM Cache (~/.npm)",
					SizeBytes:   size,
					LastModDays: age,
				})
				report.TotalBytes += size
			}
		}
	}

	// 2. Scan ~/Workspace for inactive node_modules (> 30 days old)
	wsDir := n.workspaceDir
	if wsDir == "" {
		wsDir = filepath.Join(home, "Workspace")
	}

	if info, err := os.Stat(wsDir); err == nil && info.IsDir() {
		// Walk with strict guard: do not recurse into .git or deeper into found node_modules
		_ = filepath.WalkDir(wsDir, func(path string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return nil
			}

			// Do not enter .git directories
			if d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}

			// If it's a node_modules directory
			if d.IsDir() && d.Name() == "node_modules" {
				// Safety check: ensure not in forbidden list
				if err := IsSafePath(path); err != nil {
					return filepath.SkipDir
				}

				dirInfo, err := d.Info()
				if err == nil {
					modTime := dirInfo.ModTime()
					days := int(time.Since(modTime).Hours() / 24)

					// Only target if older than 30 days
					if days >= 30 {
						size, _ := DirSize(path)
						if size > 0 {
							report.Items = append(report.Items, ItemDetail{
								ID:          "node-modules:" + path,
								Path:        path,
								Description: fmt.Sprintf("Inactive node_modules (%d days inactive)", days),
								SizeBytes:   size,
								LastModDays: days,
							})
							report.TotalBytes += size
						}
					}
				}

				// Skip traversing deeper inside this node_modules to avoid hanging on massive trees
				return filepath.SkipDir
			}

			return nil
		})
	}

	return report, nil
}

func (n *NodePlugin) Clean(itemIDs []string) (int64, error) {
	report, err := n.Scan()
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

		if err := SafeRemoveAll(item.Path); err != nil {
			return freedBytes, fmt.Errorf("failed to clean %s: %w", item.Path, err)
		}
		freedBytes += item.SizeBytes
	}

	return freedBytes, nil
}
