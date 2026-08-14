package exec

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestOSRunnerCapturesOutput(t *testing.T) {
	var out bytes.Buffer
	err := OSRunner{}.Run(context.Background(), Command{
		Name: "sh", Args: []string{"-c", "echo hello"}, Out: &out,
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "hello" {
		t.Fatalf("out = %q, want hello", out.String())
	}
}

func TestOSRunnerNonZeroIsError(t *testing.T) {
	err := OSRunner{}.Run(context.Background(), Command{
		Name: "sh", Args: []string{"-c", "exit 3"},
	})
	if err == nil {
		t.Fatal("expected error for non-zero exit")
	}
}

func TestOSRunnerPassesEnv(t *testing.T) {
	var out bytes.Buffer
	err := OSRunner{}.Run(context.Background(), Command{
		Name: "sh", Args: []string{"-c", "echo $VERSION"},
		Env:  []string{"VERSION=v9.9.9"}, Out: &out,
	})
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	if strings.TrimSpace(out.String()) != "v9.9.9" {
		t.Fatalf("out = %q, want v9.9.9", out.String())
	}
}

func TestFakeRunnerRecordsAndResponds(t *testing.T) {
	f := &FakeRunner{Handler: func(c Command) (string, error) {
		return "canned", nil
	}}
	var out bytes.Buffer
	_ = f.Run(context.Background(), Command{Name: "docker", Args: []string{"compose", "ps"}, Out: &out})
	if len(f.Calls) != 1 || f.Calls[0].Name != "docker" {
		t.Fatalf("calls not recorded: %+v", f.Calls)
	}
	if out.String() != "canned" {
		t.Fatalf("out = %q, want canned", out.String())
	}
}
