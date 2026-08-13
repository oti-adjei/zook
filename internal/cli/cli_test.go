package cli

import (
	"bytes"
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
