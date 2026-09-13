package plugins

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

type NodePlugin struct {
	workspaceDir string
	npmCacheDir  string
	inactiveDays int
}

func NewNodePlugin() *NodePlugin {
	home, _ := os.UserHomeDir()
	ws := filepath.Join(home, "Workspace")
	return &NodePlugin{
		workspaceDir: ws,
		npmCacheDir:  filepath.Join(home, ".npm"),
		inactiveDays: 30,
	}
}

func NewNodePluginWithConfig(workspaceDir string, inactiveDays int) *NodePlugin {
	if inactiveDays < 1 {
		inactiveDays = 30
	}
	home, _ := os.UserHomeDir()
	return &NodePlugin{workspaceDir: workspaceDir, npmCacheDir: filepath.Join(home, ".npm"), inactiveDays: inactiveDays}
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
	return fmt.Sprintf("Removes node_modules unchanged for at least %d days and the global npm cache. Dependencies are reinstallable.", n.inactiveDays)
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

func (n *NodePlugin) Scan(ctx context.Context) (PluginReport, error) {
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
	npmCache := n.npmCacheDir
	if npmCache == "" {
		npmCache = filepath.Join(home, ".npm")
	}
	if info, err := os.Stat(npmCache); err == nil && info.IsDir() {
		if err := IsSafePath(npmCache); err == nil {
			size, sizeErr := DirSizeContext(ctx, npmCache)
			if sizeErr != nil {
				return report, sizeErr
			}
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
		walkErr := filepath.WalkDir(wsDir, func(path string, d fs.DirEntry, entryErr error) error {
			if err := ctx.Err(); err != nil {
				return err
			}
			if entryErr != nil {
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

					if days >= n.inactiveDays {
						size, sizeErr := DirSizeContext(ctx, path)
						if sizeErr != nil {
							return sizeErr
						}
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
		if walkErr != nil {
			return report, walkErr
		}
	}

	return report, nil
}

func (n *NodePlugin) Clean(itemIDs []string) (int64, error) {
	report, err := n.Scan(context.Background())
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

		allowedRoot := n.workspaceDir
		if allowedRoot == "" {
			home, _ := os.UserHomeDir()
			allowedRoot = filepath.Join(home, "Workspace")
		}
		if id == "npm-cache" {
			allowedRoot = n.npmCacheDir
			if allowedRoot == "" {
				allowedRoot, _ = os.UserHomeDir()
				allowedRoot = filepath.Join(allowedRoot, ".npm")
			}
		}
		if err := SafeRemoveAll(item.Path, allowedRoot); err != nil {
			return freedBytes, fmt.Errorf("failed to clean %s: %w", item.Path, err)
		}
		freedBytes += FreedBytesAfterCleanup(item.Path, item.SizeBytes)
	}

	return freedBytes, nil
}
