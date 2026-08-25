package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeEnv(t *testing.T, workingDir string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), ".env")
	content := "TELEGRAM_BOT_TOKEN=test-token\nALLOWED_USER_IDS=123\nCONTINUE_COMMAND=echo hi\nNEW_COMMAND=echo new\nWORKING_DIR=" + workingDir + "\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestLoadRejectsNonexistentWorkingDir(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "does-not-exist")
	_, err := Load(writeEnv(t, missing))
	if err == nil {
		t.Fatal("expected error for non-existent WORKING_DIR")
	}
	if !strings.Contains(err.Error(), "WORKING_DIR does not exist or is not a directory") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoadAcceptsExistingWorkingDir(t *testing.T) {
	dir := t.TempDir()
	cfg, err := Load(writeEnv(t, dir))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Get().WorkingDir != dir {
		t.Fatalf("got %q, want %q", cfg.Get().WorkingDir, dir)
	}
}

func TestSetWorkingDirStillValidates(t *testing.T) {
	cfg, err := Load(writeEnv(t, t.TempDir()))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cfg.SetWorkingDir(filepath.Join(t.TempDir(), "missing")); err == nil {
		t.Fatal("expected error for non-existent directory")
	}
	file := filepath.Join(t.TempDir(), "file.txt")
	os.WriteFile(file, []byte("x"), 0o600)
	if _, err := cfg.SetWorkingDir(file); err == nil {
		t.Fatal("expected error for non-directory path")
	}
}
