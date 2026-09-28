package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestRunNoArgsPrintsUsage(t *testing.T) {
	var out, errbuf bytes.Buffer
	code := Run(nil, &out, &errbuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errbuf.String(), "usage: zook") {
		t.Fatalf("stderr = %q, want it to contain usage", errbuf.String())
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var out, errbuf bytes.Buffer
	code := Run([]string{"frobnicate"}, &out, &errbuf)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errbuf.String(), "unknown command") {
		t.Fatalf("stderr = %q, want unknown command", errbuf.String())
	}
}

func TestListPrintsDiscoveredStacks(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "alpha"), 0o755)
	os.WriteFile(filepath.Join(root, "alpha", "compose.yaml"), []byte("services: {}\n"), 0o644)
	t.Setenv("ZOOK_STACKS_ROOT", root)

	var out, errbuf bytes.Buffer
	code := Run([]string{"list"}, &out, &errbuf)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errbuf.String())
	}
	if !strings.Contains(out.String(), "alpha") {
		t.Fatalf("out=%q, want alpha", out.String())
	}
}

func TestDeployRequiresTwoArgs(t *testing.T) {
	var out, errbuf bytes.Buffer
	code := Run([]string{"deploy", "onlystack"}, &out, &errbuf)
	if code != 2 {
		t.Fatalf("exit=%d, want 2", code)
	}
	if !strings.Contains(errbuf.String(), "usage: zook deploy") {
		t.Fatalf("stderr=%q", errbuf.String())
	}
}

func TestStatusRejectsExtraArgs(t *testing.T) {
	var out, errbuf bytes.Buffer
	code := Run([]string{"status", "a", "b"}, &out, &errbuf)
	if code != 2 {
		t.Fatalf("exit=%d, want 2", code)
	}
	if !strings.Contains(errbuf.String(), "usage: zook status") {
		t.Fatalf("stderr=%q", errbuf.String())
	}
}

func TestReleasesTraversalReturnsExitOne(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZOOK_STACKS_ROOT", root)

	var out, errbuf bytes.Buffer
	code := Run([]string{"releases", "../../foo"}, &out, &errbuf)
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (invalid/nonexistent stack should error); stderr=%s", code, errbuf.String())
	}
}

func TestLogsTraversalReturnsExitOne(t *testing.T) {
	root := t.TempDir()
	t.Setenv("ZOOK_STACKS_ROOT", root)

	var out, errbuf bytes.Buffer
	code := Run([]string{"logs", "../../foo"}, &out, &errbuf)
	if code != 1 {
		t.Fatalf("exit=%d, want 1 (invalid/nonexistent stack should error); stderr=%s", code, errbuf.String())
	}
}

func TestListShowsRuntimeColumn(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "web"), 0o755)
	os.WriteFile(filepath.Join(root, "web", "compose.yaml"), []byte("services: {}\n"), 0o644)
	os.MkdirAll(filepath.Join(root, "rue"), 0o755)
	os.WriteFile(filepath.Join(root, "rue", "zook.yaml"),
		[]byte("runtime: native\nbinary: rue-api\nsystemd_unit: rue-api\nhealth:\n  url: http://localhost:8080/healthz\n"), 0o644)
	t.Setenv("ZOOK_STACKS_ROOT", root)

	var out, errbuf bytes.Buffer
	code := Run([]string{"list"}, &out, &errbuf)
	if code != 0 {
		t.Fatalf("exit=%d stderr=%s", code, errbuf.String())
	}
	s := out.String()
	if !strings.Contains(s, "RUNTIME") || !strings.Contains(s, "native") || !strings.Contains(s, "docker") {
		t.Fatalf("list should show runtime column with native+docker; got:\n%s", s)
	}
}

func TestVersionCommand(t *testing.T) {
	var out, errbuf bytes.Buffer
	code := Run([]string{"version"}, &out, &errbuf)
	if code != 0 {
		t.Fatalf("version exit = %d, want 0 (stderr: %s)", code, errbuf.String())
	}
	if !strings.HasPrefix(out.String(), "zook ") {
		t.Fatalf("version output = %q, want it to start with \"zook \"", out.String())
	}
}

func TestServeUsageError(t *testing.T) {
	var out, errbuf bytes.Buffer
	if code := Run([]string{"serve", "extra"}, &out, &errbuf); code != 2 {
		t.Fatalf("serve with args → 2, got %d", code)
	}
}

func TestServeBadAddrFails(t *testing.T) {
	t.Setenv("ZOOK_ADDR", "256.256.256.256:99999") // invalid → immediate listen error
	var out, errbuf bytes.Buffer
	// Give the listener a moment; cmdServe exits 1 on ListenAndServe failure.
	done := make(chan int, 1)
	go func() { done <- Run([]string{"serve"}, &out, &errbuf) }()
	select {
	case code := <-done:
		if code != 1 {
			t.Fatalf("invalid addr → 1, got %d (stderr: %s)", code, errbuf.String())
		}
	case <-time.After(3 * time.Second):
		t.Fatal("serve did not exit on listen error")
	}
}
