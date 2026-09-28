package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeZook(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ZookFileName), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestLoadConfigMissingIsNil(t *testing.T) {
	c, err := LoadConfig(t.TempDir())
	if err != nil || c != nil {
		t.Fatalf("missing zook.yaml → (nil,nil); got (%v,%v)", c, err)
	}
}

func TestLoadConfigNativeValid(t *testing.T) {
	dir := t.TempDir()
	writeZook(t, dir, `
runtime: native
binary: rue-api
systemd_unit: rue-api
artifact: https://ci.example.com/rue-${VERSION}.tar.gz
health:
  url: http://localhost:8080/healthz
  expected_status: 200
health_timeout: 45s
rollback_on_fail: false
`)
	c, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.ResolvedRuntime() != "native" || c.Binary != "rue-api" || c.SystemdUnit != "rue-api" {
		t.Fatalf("bad parse: %+v", c)
	}
	if c.Health == nil || c.Health.URL == "" {
		t.Fatalf("health not parsed: %+v", c)
	}
	if got := c.Timeout(60 * time.Second); got != 45*time.Second {
		t.Fatalf("timeout = %v, want 45s", got)
	}
	if c.ShouldRollback() != false {
		t.Fatalf("rollback_on_fail should be false")
	}
}

func TestResolvedRuntimeDefaults(t *testing.T) {
	var nilC *StackConfig
	if nilC.ResolvedRuntime() != "docker" {
		t.Fatal("nil config → docker")
	}
	dir := t.TempDir()
	writeZook(t, dir, "health_timeout: 30s\n")
	c, _ := LoadConfig(dir)
	if c.ResolvedRuntime() != "docker" {
		t.Fatalf("empty runtime → docker, got %q", c.ResolvedRuntime())
	}
	if c.ShouldRollback() != true {
		t.Fatal("rollback default true")
	}
}

func TestLoadConfigRejectsUnknownKey(t *testing.T) {
	dir := t.TempDir()
	writeZook(t, dir, "runtme: native\n") // typo
	if _, err := LoadConfig(dir); err == nil {
		t.Fatal("unknown key must error")
	}
}

func TestValidateNativeRequirements(t *testing.T) {
	cases := map[string]string{
		"missing binary":       "runtime: native\nsystemd_unit: u\nhealth:\n  url: http://x/h\n",
		"missing unit":         "runtime: native\nbinary: b\nhealth:\n  url: http://x/h\n",
		"missing health":       "runtime: native\nbinary: b\nsystemd_unit: u\n",
		"both url and command": "runtime: native\nbinary: b\nsystemd_unit: u\nhealth:\n  url: http://x/h\n  command: [b, health]\n",
		"bad timeout":          "runtime: native\nbinary: b\nsystemd_unit: u\nhealth:\n  url: http://x/h\nhealth_timeout: nope\n",
		"bad runtime":          "runtime: podman\n",
	}
	for name, body := range cases {
		dir := t.TempDir()
		writeZook(t, dir, body)
		if _, err := LoadConfig(dir); err == nil {
			t.Fatalf("%s: expected validation error", name)
		}
	}
}

func TestValidateNativeCommandOK(t *testing.T) {
	dir := t.TempDir()
	writeZook(t, dir, "runtime: native\nbinary: b\nsystemd_unit: u\nhealth:\n  command: [b, health]\n")
	if _, err := LoadConfig(dir); err != nil {
		t.Fatalf("command health should be valid: %v", err)
	}
}

func TestDeployOnValidBranch(t *testing.T) {
	dir := t.TempDir()
	writeZook(t, dir, "deploy_on:\n  github:\n    repo: oti-adjei/rue\n    branch: main\n")
	c, err := LoadConfig(dir)
	if err != nil {
		t.Fatalf("valid deploy_on should parse: %v", err)
	}
	if c.DeployOn == nil || c.DeployOn.GitHub == nil || c.DeployOn.GitHub.Branch != "main" {
		t.Fatalf("deploy_on not parsed: %+v", c.DeployOn)
	}
}

func TestDeployOnValidTagGlob(t *testing.T) {
	dir := t.TempDir()
	writeZook(t, dir, "runtime: native\nbinary: b\nsystemd_unit: u\nhealth:\n  command: [b]\ndeploy_on:\n  github:\n    repo: oti-adjei/rue\n    tag: \"v*\"\n")
	c, err := LoadConfig(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.DeployOn.GitHub.Tag != "v*" {
		t.Fatalf("tag glob not parsed: %+v", c.DeployOn)
	}
}

func TestDeployOnRejections(t *testing.T) {
	cases := map[string]string{
		"missing repo":    "deploy_on:\n  github:\n    branch: main\n",
		"both branch+tag": "deploy_on:\n  github:\n    repo: o/r\n    branch: main\n    tag: \"v*\"\n",
		"neither":         "deploy_on:\n  github:\n    repo: o/r\n",
	}
	for name, body := range cases {
		dir := t.TempDir()
		writeZook(t, dir, body)
		if _, err := LoadConfig(dir); err == nil {
			t.Fatalf("%s: expected error", name)
		}
	}
}

func TestDeployOnAbsentIsFine(t *testing.T) {
	dir := t.TempDir()
	writeZook(t, dir, "health_timeout: 30s\n")
	c, err := LoadConfig(dir)
	if err != nil || c.DeployOn != nil {
		t.Fatalf("no deploy_on → nil: %v %+v", err, c.DeployOn)
	}
}
