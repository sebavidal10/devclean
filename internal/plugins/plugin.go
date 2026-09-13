package plugins

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// ItemDetail represents a specific file, directory, or resource detected by a plugin.
type ItemDetail struct {
	ID          string `json:"id"`
	Path        string `json:"path"`
	Description string `json:"description"`
	SizeBytes   int64  `json:"size_bytes"`
	LastModDays int    `json:"last_mod_days"`
}

// PluginReport aggregates detected recoverable items and metrics for a specific plugin.
type PluginReport struct {
	PluginID   string       `json:"plugin_id"`
	Category   string       `json:"category"`
	Title      string       `json:"title"`
	SafetyNote string       `json:"safety_note"`
	TotalBytes int64        `json:"total_bytes"`
	Items      []ItemDetail `json:"items"`
}

// CleanerPlugin defines the contract for any cleanable development tool or environment.
type CleanerPlugin interface {
	ID() string
	Category() string
	Name() string
	SafetyNote() string
	Detect() bool
	Scan(context.Context) (PluginReport, error)
	Clean(itemIDs []string) (freedBytes int64, err error)
}

// Registry manages plugins and coordinates concurrent scanning and execution.
type Registry struct {
	mu      sync.RWMutex
	plugins []CleanerPlugin
}

// NewRegistry initializes an empty plugin registry.
func NewRegistry() *Registry {
	return &Registry{
		plugins: make([]CleanerPlugin, 0),
	}
}

// Register adds a CleanerPlugin to the registry.
func (r *Registry) Register(plugin CleanerPlugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = append(r.plugins, plugin)
}

// Plugins returns a copy of registered plugins.
func (r *Registry) Plugins() []CleanerPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	copied := make([]CleanerPlugin, len(r.plugins))
	copy(copied, r.plugins)
	return copied
}

// Find returns the registered plugin with the specified ID, or nil if not found.
func (r *Registry) Find(id string) CleanerPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, p := range r.plugins {
		if p.ID() == id {
			return p
		}
	}
	return nil
}

type scanResult struct {
	report PluginReport
	err    error
}

// ScanAll runs Detect() on all registered plugins and scans the detected ones concurrently
// using sync.WaitGroup, worker goroutines, and channels, honoring context timeout/cancellation.
func (r *Registry) ScanAll(ctx context.Context) ([]PluginReport, error) {
	plugins := r.Plugins()
	var detected []CleanerPlugin

	for _, p := range plugins {
		if p.Detect() {
			detected = append(detected, p)
		}
	}

	if len(detected) == 0 {
		return []PluginReport{}, nil
	}

	resultCh := make(chan scanResult, len(detected))
	var wg sync.WaitGroup

	for _, p := range detected {
		wg.Add(1)
		go func(plugin CleanerPlugin) {
			defer wg.Done()

			rep, err := plugin.Scan(ctx)
			if err != nil {
				err = fmt.Errorf("scan %s: %w", plugin.Name(), err)
			}
			resultCh <- scanResult{report: rep, err: err}
		}(p)
	}

	// Close result channel when all workers finish
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var reports []PluginReport
	var scanErrors []error

	for res := range resultCh {
		if res.err != nil {
			scanErrors = append(scanErrors, res.err)
			continue
		}
		sort.Slice(res.report.Items, func(i, j int) bool {
			return res.report.Items[i].ID < res.report.Items[j].ID
		})
		reports = append(reports, res.report)
	}
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].PluginID < reports[j].PluginID
	})

	if ctx.Err() != nil {
		return reports, ctx.Err()
	}

	return reports, errors.Join(scanErrors...)
}

// DirSize safely computes the total bytes of all files in a directory tree without following symlinks.
// It skips .git directories and avoids reading forbidden file types.
func DirSize(root string) (int64, error) {
	return DirSizeContext(context.Background(), root)
}

// DirSizeContext is DirSize with cooperative cancellation for large trees.
func DirSizeContext(ctx context.Context, root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err != nil {
			// Skip permission denied or inaccessible files gracefully
			return nil
		}
		if d.IsDir() && d.Name() == ".git" {
			return filepath.SkipDir
		}
		if !d.IsDir() {
			info, err := d.Info()
			if err == nil {
				total += info.Size()
			}
		}
		return nil
	})
	return total, err
}

// PathAgeDays calculates how many days have elapsed since the file or directory was last modified.
func PathAgeDays(path string) int {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	duration := time.Since(info.ModTime())
	days := int(duration.Hours() / 24)
	if days < 0 {
		return 0
	}
	return days
}

// FreedBytesAfterCleanup calculates actual reclaimed bytes when protected descendants remain.
func FreedBytesAfterCleanup(path string, before int64) int64 {
	after, err := DirSize(path)
	if err != nil {
		if os.IsNotExist(err) {
			return before
		}
		return 0
	}
	if after >= before {
		return 0
	}
	return before - after
}
