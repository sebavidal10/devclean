package plugins

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"strings"
)

var (
	ErrEmptyPath          = errors.New("safety violation: path cannot be empty")
	ErrRootPath           = errors.New("safety violation: target path cannot be system root or system directory")
	ErrUserHomeRoot       = errors.New("safety violation: target path cannot be user home root")
	ErrGitDirectory       = errors.New("safety violation: cannot target .git repositories")
	ErrEnvFile            = errors.New("safety violation: cannot target environment files (.env*)")
	ErrDatabaseFile       = errors.New("safety violation: cannot target SQLite or database files (*.db, *.sqlite*)")
	ErrSymlinkTargetRisk  = errors.New("safety violation: cannot delete symlinks targeting outside cache boundaries")
	ErrNoAllowedRoot      = errors.New("safety violation: at least one allowed cleanup root is required")
	ErrOutsideAllowedRoot = errors.New("safety violation: target is outside allowed cleanup roots")
)

// protectedSystemRoots contains paths that should never under any circumstances be deleted.
var protectedSystemRoots = []string{
	"/",
	"/Applications",
	"/Library",
	"/System",
	"/Users",
	"/bin",
	"/dev",
	"/etc",
	"/opt",
	"/private",
	"/sbin",
	"/tmp",
	"/usr",
	"/var",
}

// IsSafePath verifies that a target path does not violate any Zero-Footgun safety rules:
// 1. Not empty or relative root.
// 2. Not a critical system or user home directory.
// 3. Does not contain or touch .git/, .env*, *.db, or *.sqlite*.
func IsSafePath(p string) error {
	if strings.TrimSpace(p) == "" {
		return ErrEmptyPath
	}

	cleanPath := filepath.Clean(p)

	// Check system roots
	for _, sysRoot := range protectedSystemRoots {
		if cleanPath == sysRoot {
			return fmt.Errorf("%w: %s", ErrRootPath, cleanPath)
		}
	}

	// Check user home directory
	currUser, err := user.Current()
	if err == nil && currUser.HomeDir != "" {
		cleanHome := filepath.Clean(currUser.HomeDir)
		if cleanPath == cleanHome {
			return fmt.Errorf("%w: %s", ErrUserHomeRoot, cleanPath)
		}
	}

	// Inspect all segments of the path
	parts := strings.Split(cleanPath, string(filepath.Separator))
	for _, part := range parts {
		if part == "" {
			continue
		}

		// Rule: Prohibido listar o eliminar directorios .git/
		if part == ".git" {
			return fmt.Errorf("%w: %s", ErrGitDirectory, cleanPath)
		}

		// Rule: Archivos de entorno .env*
		if strings.HasPrefix(part, ".env") {
			return fmt.Errorf("%w: %s", ErrEnvFile, cleanPath)
		}

		// Rule: Bases de datos SQLite locales (*.db, *.sqlite*)
		lowerPart := strings.ToLower(part)
		if strings.HasSuffix(lowerPart, ".db") ||
			strings.HasSuffix(lowerPart, ".sqlite") ||
			strings.HasSuffix(lowerPart, ".sqlite3") ||
			strings.HasSuffix(lowerPart, ".sqlite-wal") ||
			strings.HasSuffix(lowerPart, ".sqlite-shm") {
			return fmt.Errorf("%w: %s", ErrDatabaseFile, cleanPath)
		}
	}

	return nil
}

// SafeRemoveAll removes safe entries beneath an explicitly authorized cleanup root.
// Protected descendants are preserved, and symlinks are unlinked without being followed.
func SafeRemoveAll(p string, allowedRoots ...string) error {
	if err := IsSafePath(p); err != nil {
		return err
	}
	if len(allowedRoots) == 0 {
		return ErrNoAllowedRoot
	}

	target, err := filepath.Abs(p)
	if err != nil {
		return fmt.Errorf("failed to resolve target path %s: %w", p, err)
	}
	allowed := false
	for _, root := range allowedRoots {
		rootAbs, absErr := filepath.Abs(root)
		if absErr != nil {
			continue
		}
		rel, relErr := filepath.Rel(rootAbs, target)
		if relErr == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			allowed = true
			break
		}
	}
	if !allowed {
		return fmt.Errorf("%w: %s", ErrOutsideAllowedRoot, target)
	}

	return removeSafeTree(target)
}

func removeSafeTree(p string) error {
	info, err := os.Lstat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Nothing to delete
		}
		return fmt.Errorf("failed to inspect path %s: %w", p, err)
	}

	if info.Mode()&os.ModeSymlink != 0 || !info.IsDir() {
		return os.Remove(p)
	}

	entries, err := os.ReadDir(p)
	if err != nil {
		return fmt.Errorf("failed to read directory %s: %w", p, err)
	}
	for _, entry := range entries {
		child := filepath.Join(p, entry.Name())
		if err := IsSafePath(child); err != nil {
			continue
		}
		if err := removeSafeTree(child); err != nil {
			return err
		}
	}
	if err := os.Remove(p); err != nil && !errors.Is(err, os.ErrNotExist) {
		// A protected descendant intentionally leaves the directory non-empty.
		if entries, readErr := os.ReadDir(p); readErr == nil && len(entries) > 0 {
			return nil
		}
		return err
	}
	return nil
}
