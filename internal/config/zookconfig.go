package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// ZookFileName is the per-stack config file zook looks for.
const ZookFileName = "zook.yaml"

// HealthConfig declares how zook decides a native stack is healthy.
// Exactly one of URL / Command must be set.
type HealthConfig struct {
	URL            string   `yaml:"url"`
	Command        []string `yaml:"command"`
	ExpectedStatus int      `yaml:"expected_status"`
}

// GitHubDeploy wires a stack to GitHub push events. Branch matches exactly;
// Tag is a glob (path.Match syntax, e.g. "v*"). Exactly one of Branch/Tag
// must be set.
type GitHubDeploy struct {
	Repo   string `yaml:"repo"`
	Branch string `yaml:"branch"`
	Tag    string `yaml:"tag"`
}

// DeployOnConfig declares automatic deploy triggers.
type DeployOnConfig struct {
	GitHub *GitHubDeploy `yaml:"github"`
}

// StackConfig is a parsed zook.yaml. Optional for docker, required for native.
type StackConfig struct {
	Runtime        string          `yaml:"runtime"`
	Binary         string          `yaml:"binary"`
	SystemdUnit    string          `yaml:"systemd_unit"`
	Artifact       string          `yaml:"artifact"`
	Health         *HealthConfig   `yaml:"health"`
	HealthTimeout  string          `yaml:"health_timeout"`
	RollbackOnFail *bool           `yaml:"rollback_on_fail"`
	DeployOn       *DeployOnConfig `yaml:"deploy_on"`
}

// LoadConfig reads and validates <dir>/zook.yaml. A missing file yields (nil, nil).
func LoadConfig(dir string) (*StackConfig, error) {
	data, err := os.ReadFile(filepath.Join(dir, ZookFileName))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	dec := yaml.NewDecoder(bytesReader(data))
	dec.KnownFields(true)
	var c StackConfig
	if err := dec.Decode(&c); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ZookFileName, err)
	}
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", ZookFileName, err)
	}
	return &c, nil
}

// ResolvedRuntime returns "native" or "docker" (the default).
func (c *StackConfig) ResolvedRuntime() string {
	if c == nil || c.Runtime == "" {
		return "docker"
	}
	return c.Runtime
}

// Validate enforces the runtime value and native field requirements.
func (c *StackConfig) Validate() error {
	switch c.Runtime {
	case "", "docker":
		// docker stacks need no zook.yaml fields
	case "native":
		if c.Binary == "" {
			return errors.New("native runtime requires 'binary'")
		}
		if c.SystemdUnit == "" {
			return errors.New("native runtime requires 'systemd_unit'")
		}
		if c.Health == nil {
			return errors.New("native runtime requires a 'health' block")
		}
		hasURL := c.Health.URL != ""
		hasCmd := len(c.Health.Command) > 0
		if hasURL == hasCmd {
			return errors.New("health must set exactly one of 'url' or 'command'")
		}
	default:
		return fmt.Errorf("unknown runtime %q (want docker or native)", c.Runtime)
	}
	if c.DeployOn != nil && c.DeployOn.GitHub != nil {
		g := c.DeployOn.GitHub
		if g.Repo == "" {
			return errors.New("deploy_on.github requires 'repo'")
		}
		if (g.Branch == "") == (g.Tag == "") {
			return errors.New("deploy_on.github must set exactly one of 'branch' or 'tag'")
		}
	}
	if c.HealthTimeout != "" {
		if _, err := time.ParseDuration(c.HealthTimeout); err != nil {
			return fmt.Errorf("invalid health_timeout %q: %w", c.HealthTimeout, err)
		}
	}
	return nil
}

// Timeout returns the configured health timeout, or def when unset.
func (c *StackConfig) Timeout(def time.Duration) time.Duration {
	if c == nil || c.HealthTimeout == "" {
		return def
	}
	d, err := time.ParseDuration(c.HealthTimeout) // validated in Validate
	if err != nil {
		return def
	}
	return d
}

// ShouldRollback reports rollback_on_fail, defaulting to true.
func (c *StackConfig) ShouldRollback() bool {
	if c == nil || c.RollbackOnFail == nil {
		return true
	}
	return *c.RollbackOnFail
}

func bytesReader(b []byte) *bytes.Reader { return bytes.NewReader(b) }
