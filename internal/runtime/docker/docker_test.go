package docker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/exec"
)

func stack() config.Stack {
	return config.Stack{Name: "app", Dir: "/opt/stacks/app", ComposeFile: "/opt/stacks/app/compose.yaml"}
}

func joined(c exec.Command) string { return c.Name + " " + strings.Join(c.Args, " ") }

func TestPreflightFlagsMissingHealthcheck(t *testing.T) {
	f := &exec.FakeRunner{Handler: func(c exec.Command) (string, error) {
		cmd := joined(c)
		switch {
		case strings.Contains(cmd, "config --services"):
			return "core\ntenant-api\n", nil
		case strings.Contains(cmd, "config --format json"):
			return `{"services":{"core":{"healthcheck":{"test":["CMD","true"]}},"tenant-api":{}}}`, nil
		}
		return "", nil
	}}
	missing, err := New(f).Preflight(context.Background(), stack(), nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 1 || missing[0] != "tenant-api" {
		t.Fatalf("missing = %v, want [tenant-api]", missing)
	}
}

func TestUpBuildsWaitCommand(t *testing.T) {
	f := &exec.FakeRunner{}
	err := New(f).Up(context.Background(), stack(), "v1.2.3", 45*time.Second, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := joined(f.Calls[0])
	for _, want := range []string{"docker compose", "-f /opt/stacks/app/compose.yaml", "up -d", "--wait", "--wait-timeout 45"} {
		if !strings.Contains(got, want) {
			t.Fatalf("command %q missing %q", got, want)
		}
	}
	foundVersion := false
	for _, e := range f.Calls[0].Env {
		if e == "VERSION=v1.2.3" {
			foundVersion = true
		}
	}
	if !foundVersion {
		t.Fatalf("VERSION not in env: %v", f.Calls[0].Env)
	}
}

func TestHealthAllRunning(t *testing.T) {
	f := &exec.FakeRunner{Handler: func(c exec.Command) (string, error) {
		return `{"Service":"core","State":"running","Health":"healthy"}
{"Service":"pg","State":"running","Health":""}`, nil
	}}
	ok, err := New(f).Health(context.Background(), stack())
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected healthy")
	}
}

func TestHealthDetectsUnhealthy(t *testing.T) {
	f := &exec.FakeRunner{Handler: func(c exec.Command) (string, error) {
		return `{"Service":"core","State":"running","Health":"unhealthy"}`, nil
	}}
	ok, _ := New(f).Health(context.Background(), stack())
	if ok {
		t.Fatal("expected not healthy")
	}
}
