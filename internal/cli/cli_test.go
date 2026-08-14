package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
