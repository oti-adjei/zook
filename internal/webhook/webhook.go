// Package webhook verifies and parses GitHub webhook deliveries (push
// events). It is pure: HMAC verification against the raw body, payload
// decoding, and matching a push against a stack's deploy_on.github config.
package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/oti-adjei/zook/internal/config"
)

// Verify checks a GitHub X-Hub-Signature-256 header ("sha256=<hex>") against
// the raw request body using the shared secret.
func Verify(secret, body []byte, sigHeader string) bool {
	const prefix = "sha256="
	if !strings.HasPrefix(sigHeader, prefix) {
		return false
	}
	given, err := hex.DecodeString(strings.TrimPrefix(sigHeader, prefix))
	if err != nil {
		return false
	}
	m := hmac.New(sha256.New, secret)
	m.Write(body)
	return hmac.Equal(given, m.Sum(nil))
}

// PushEvent is the subset of a GitHub push payload zook cares about.
type PushEvent struct {
	Ref   string
	After string
	Repo  string
}

type pushPayload struct {
	Ref        string `json:"ref"`
	After      string `json:"after"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

// ParsePush decodes a push-event body.
func ParsePush(body []byte) (PushEvent, error) {
	var p pushPayload
	if err := json.Unmarshal(body, &p); err != nil {
		return PushEvent{}, fmt.Errorf("parsing push payload: %w", err)
	}
	return PushEvent{Ref: p.Ref, After: p.After, Repo: p.Repository.FullName}, nil
}

// TagName returns the pushed tag, or "" for non-tag refs.
func (e PushEvent) TagName() string {
	if strings.HasPrefix(e.Ref, "refs/tags/") {
		return strings.TrimPrefix(e.Ref, "refs/tags/")
	}
	return ""
}

// BranchName returns the pushed branch, or "" for non-branch refs.
func (e PushEvent) BranchName() string {
	if strings.HasPrefix(e.Ref, "refs/heads/") {
		return strings.TrimPrefix(e.Ref, "refs/heads/")
	}
	return ""
}

// Match reports whether the event triggers g, returning the version to
// deploy: the tag name for tag pushes, or the full commit SHA for branch
// pushes. Repo must match exactly; branch matches exactly; tag matches as a
// glob (path.Match syntax).
func (e PushEvent) Match(g config.GitHubDeploy) (string, bool) {
	if e.Repo != g.Repo {
		return "", false
	}
	if t := e.TagName(); t != "" && g.Tag != "" {
		if ok, err := path.Match(g.Tag, t); err == nil && ok {
			return t, true
		}
		return "", false
	}
	if b := e.BranchName(); b != "" && g.Branch != "" && b == g.Branch {
		return e.After, true
	}
	return "", false
}
