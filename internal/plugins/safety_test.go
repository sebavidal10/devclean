package plugins

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestIsSafePath(t *testing.T) {
	currUser, _ := user.Current()
	homeDir := currUser.HomeDir

	tests := []struct {
		name        string
		path        string
		expectError error
	}{
		{"empty path", "", ErrEmptyPath},
		{"root path", "/", ErrRootPath},
		{"system library", "/Library", ErrRootPath},
		{"system bin", "/bin", ErrRootPath},
		{"user home root", homeDir, ErrUserHomeRoot},
		{".git directory", "/Users/dev/project/.git", ErrGitDirectory},
		{".git subpath", "/Users/dev/project/.git/objects", ErrGitDirectory},
		{".env file", "/Users/dev/project/.env", ErrEnvFile},
		{".env.local file", "/Users/dev/project/.env.local", ErrEnvFile},
		{".env.production file", "/Users/dev/project/.env.production", ErrEnvFile},
		{"sqlite db file", "/Users/dev/project/data.db", ErrDatabaseFile},
		{"sqlite3 file", "/Users/dev/project/app.sqlite3", ErrDatabaseFile},
		{"sqlite-wal file", "/Users/dev/project/app.sqlite-wal", ErrDatabaseFile},
		{"valid derived data", homeDir + "/Library/Developer/Xcode/DerivedData/MyApp-abcd", nil},
		{"valid node modules", "/Users/dev/Workspace/project/node_modules", nil},
		{"valid npm cache", homeDir + "/.npm/_cacache", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := IsSafePath(tt.path)
			if tt.expectError != nil {
				if err == nil {
					t.Fatalf("expected error containing %v, got nil", tt.expectError)
				}
				if !errors.Is(err, tt.expectError) {
					t.Fatalf("expected error is %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Fatalf("expected path to be safe, got error: %v", err)
				}
			}
		})
	}
}

func TestSafeRemoveAllProtection(t *testing.T) {
	// Create a temporary sandbox directory to simulate user project
	tmpDir, err := os.MkdirTemp("", "devclean-safety-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Create a simulated .git directory
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.Mkdir(gitDir, 0755); err != nil {
		t.Fatalf("failed to create fake .git: %v", err)
	}
	gitFile := filepath.Join(gitDir, "HEAD")
	if err := os.WriteFile(gitFile, []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatalf("failed to write git HEAD: %v", err)
	}

	// Attempt SafeRemoveAll on .git
	err = SafeRemoveAll(gitDir)
	if err == nil {
		t.Errorf("CRITICAL SAFETY FAILURE: SafeRemoveAll allowed deleting .git directory!")
	}
	if !errors.Is(err, ErrGitDirectory) {
		t.Errorf("expected ErrGitDirectory, got %v", err)
	}
	// Verify .git still exists
	if _, err := os.Stat(gitFile); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: .git file was deleted despite safety check!")
	}

	// 2. Create a simulated .env file
	envFile := filepath.Join(tmpDir, ".env.production")
	if err := os.WriteFile(envFile, []byte("SECRET_KEY=supersecret"), 0644); err != nil {
		t.Fatalf("failed to write .env: %v", err)
	}

	// Attempt SafeRemoveAll on .env
	err = SafeRemoveAll(envFile)
	if err == nil {
		t.Errorf("CRITICAL SAFETY FAILURE: SafeRemoveAll allowed deleting .env file!")
	}
	if !errors.Is(err, ErrEnvFile) {
		t.Errorf("expected ErrEnvFile, got %v", err)
	}
	// Verify .env still exists
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: .env file was deleted despite safety check!")
	}

	// 3. Create a simulated SQLite database
	dbFile := filepath.Join(tmpDir, "production.sqlite3")
	if err := os.WriteFile(dbFile, []byte("SQLite format 3\x00"), 0644); err != nil {
		t.Fatalf("failed to write sqlite db: %v", err)
	}

	// Attempt SafeRemoveAll on sqlite3
	err = SafeRemoveAll(dbFile)
	if err == nil {
		t.Errorf("CRITICAL SAFETY FAILURE: SafeRemoveAll allowed deleting SQLite file!")
	}
	if !errors.Is(err, ErrDatabaseFile) {
		t.Errorf("expected ErrDatabaseFile, got %v", err)
	}
	// Verify db still exists
	if _, err := os.Stat(dbFile); os.IsNotExist(err) {
		t.Fatalf("CRITICAL: SQLite db was deleted despite safety check!")
	}

	// 4. Create an authorized cache file to verify safe deletion works on valid targets
	cacheDir := filepath.Join(tmpDir, "cache_to_delete")
	if err := os.Mkdir(cacheDir, 0755); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}
	cacheFile := filepath.Join(cacheDir, "temp.bin")
	_ = os.WriteFile(cacheFile, []byte("temporary data"), 0644)

	err = SafeRemoveAll(cacheDir)
	if err != nil {
		t.Errorf("SafeRemoveAll failed on legitimate cache dir: %v", err)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("legitimate cache dir was not deleted")
	}
}
