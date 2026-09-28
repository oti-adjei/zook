package webhook

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/oti-adjei/zook/internal/config"
)

func sig(secret, body string) string {
	m := hmac.New(sha256.New, []byte(secret))
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

const pushBody = `{"ref":"refs/heads/main","after":"abc123def456abc123def456abc123def456abc1","repository":{"full_name":"oti-adjei/rue"}}`

func TestVerifyGoodAndBad(t *testing.T) {
	if !Verify([]byte("s3cret"), []byte(pushBody), sig("s3cret", pushBody)) {
		t.Fatal("good signature must verify")
	}
	if Verify([]byte("s3cret"), []byte(pushBody), sig("wrong", pushBody)) {
		t.Fatal("wrong secret must fail")
	}
	if Verify([]byte("s3cret"), []byte(pushBody), "garbage") {
		t.Fatal("malformed header must fail")
	}
	if Verify([]byte("s3cret"), []byte(pushBody), "") {
		t.Fatal("empty header must fail")
	}
}

func TestParsePush(t *testing.T) {
	e, err := ParsePush([]byte(pushBody))
	if err != nil {
		t.Fatal(err)
	}
	if e.Repo != "oti-adjei/rue" || e.BranchName() != "main" || e.TagName() != "" {
		t.Fatalf("parse wrong: %+v", e)
	}
	v, ok := e.Match(config.GitHubDeploy{Repo: "oti-adjei/rue", Branch: "main"})
	if !ok || v != "abc123def456abc123def456abc123def456abc1" {
		t.Fatalf("branch match should deploy the full SHA, got %q %v", v, ok)
	}
}

func TestTagMatchGlob(t *testing.T) {
	body, _ := json.Marshal(map[string]any{
		"ref": "refs/tags/v1.2.3", "after": "deadbeef",
		"repository": map[string]any{"full_name": "oti-adjei/rue"},
	})
	e, err := ParsePush(body)
	if err != nil {
		t.Fatal(err)
	}
	if e.TagName() != "v1.2.3" || e.BranchName() != "" {
		t.Fatalf("tag parse wrong: %+v", e)
	}
	if v, ok := e.Match(config.GitHubDeploy{Repo: "oti-adjei/rue", Tag: "v*"}); !ok || v != "v1.2.3" {
		t.Fatalf("tag glob should match: %q %v", v, ok)
	}
	if _, ok := e.Match(config.GitHubDeploy{Repo: "oti-adjei/rue", Tag: "release-*"}); ok {
		t.Fatal("non-matching glob must not match")
	}
}

func TestMatchRejectsOtherRepoAndRefs(t *testing.T) {
	e, _ := ParsePush([]byte(pushBody))
	if _, ok := e.Match(config.GitHubDeploy{Repo: "other/repo", Branch: "main"}); ok {
		t.Fatal("other repo must not match")
	}
	// branch event against a tag-configured stack
	if _, ok := e.Match(config.GitHubDeploy{Repo: "oti-adjei/rue", Tag: "v*"}); ok {
		t.Fatal("branch push must not match tag config")
	}
}
