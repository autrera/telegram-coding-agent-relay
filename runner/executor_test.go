package runner

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
)

func TestExecuteMissingWorkingDirReturnsError(t *testing.T) {
	e := NewExecutor()
	res := e.Execute(context.Background(), "/bin/sh -c 'echo hi'", "", filepath.Join(t.TempDir(), "missing"), "auto", nil)
	if res.Err == nil {
		t.Fatal("expected error for missing working directory")
	}
	if res.ExitCode != 1 {
		t.Fatalf("exit code = %d, want 1", res.ExitCode)
	}
	if !strings.Contains(res.Err.Error(), "failed to start process") {
		t.Fatalf("unexpected error: %v", res.Err)
	}
}

func TestExecuteSuccess(t *testing.T) {
	e := NewExecutor()
	res := e.Execute(context.Background(), "/bin/sh -c 'echo hello'", "", t.TempDir(), "auto", nil)
	if res.Err != nil || res.ExitCode != 0 {
		t.Fatalf("err=%v exit=%d", res.Err, res.ExitCode)
	}
	if !strings.Contains(res.Stdout, "hello") {
		t.Fatalf("stdout = %q", res.Stdout)
	}
}
