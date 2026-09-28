package server

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oti-adjei/zook/internal/config"
	"github.com/oti-adjei/zook/internal/core"
)

// fakeEngine records calls and can block or fail, scripted per test.
type fakeEngine struct {
	mu        sync.Mutex
	deploys   []string
	rollbacks []string
	block     chan struct{}
	err       error
}

func (f *fakeEngine) Deploy(_ context.Context, s config.Stack, version string) error {
	f.mu.Lock()
	f.deploys = append(f.deploys, s.Name+":"+version)
	block, err := f.block, f.err
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	return err
}

func (f *fakeEngine) Rollback(_ context.Context, s config.Stack) error {
	f.mu.Lock()
	f.rollbacks = append(f.rollbacks, s.Name)
	block, err := f.block, f.err
	f.mu.Unlock()
	if block != nil {
		<-block
	}
	return err
}

func (f *fakeEngine) snapshot() (deploys, rollbacks []string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.deploys...), append([]string{}, f.rollbacks...)
}

func (f *fakeEngine) failWith(err error) {
	f.mu.Lock()
	f.err = err
	f.mu.Unlock()
}

// newTestServer builds a server over a temp stacks root with two stacks:
// "web" (docker, compose only) and "rue" (zook.yaml with a tag deploy_on).
func newTestServer(t *testing.T, mutate func(*Options)) (*httptest.Server, *fakeEngine, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "web"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "web", "compose.yaml"), []byte("services: {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveState(filepath.Join(root, "web"), core.State{
		Current: "v1", Previous: "v0",
		History: []core.HistoryEntry{{Version: "v1", Timestamp: time.Now(), Result: core.ResultSuccess}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "rue"), 0o755); err != nil {
		t.Fatal(err)
	}
	zook := "deploy_on:\n  github:\n    repo: oti-adjei/rue\n    tag: \"v*\"\n"
	if err := os.WriteFile(filepath.Join(root, "rue", "zook.yaml"), []byte(zook), 0o644); err != nil {
		t.Fatal(err)
	}

	fake := &fakeEngine{}
	opts := Options{
		StacksRoot:    root,
		APIToken:      "tok",
		WebhookSecret: []byte("hooksecret"),
		NewEngine:     func(config.Stack) Deployer { return fake },
	}
	if mutate != nil {
		mutate(&opts)
	}
	ts := httptest.NewServer(New(opts).Handler())
	t.Cleanup(ts.Close)
	return ts, fake, root
}

func getJSON(t *testing.T, url string, headers map[string]string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&body)
	return resp.StatusCode, body
}

func postJSON(t *testing.T, url, payload string, headers map[string]string) (int, map[string]any) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var body map[string]any
	if resp.StatusCode != http.StatusNoContent {
		_ = json.NewDecoder(resp.Body).Decode(&body)
	}
	return resp.StatusCode, body
}

func waitJob(t *testing.T, tsURL, id string) map[string]any {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		_, body := getJSON(t, tsURL+"/jobs/"+id, nil)
		if body["status"] == "done" || body["status"] == "failed" {
			return body
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatalf("job %s never finished", id)
	return nil
}

func TestHealthz(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	code, body := getJSON(t, ts.URL+"/healthz", nil)
	if code != 200 || body["status"] != "ok" {
		t.Fatalf("healthz: %d %v", code, body)
	}
}

func TestListStacks(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	code, body := getJSON(t, ts.URL+"/stacks", nil)
	if code != 200 {
		t.Fatalf("stacks: %d", code)
	}
	stacks, ok := body["stacks"].([]any)
	if !ok || len(stacks) != 2 {
		t.Fatalf("want 2 stacks: %v", body)
	}
	first, _ := stacks[0].(map[string]any)
	if first["name"] != "rue" || first["runtime"] != "docker" {
		t.Fatalf("rue should be first, docker runtime: %v", first)
	}
}

func TestGetStackState(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	code, body := getJSON(t, ts.URL+"/stacks/web", nil)
	state, _ := body["state"].(map[string]any)
	if code != 200 || body["runtime"] != "docker" || state["current"] != "v1" || state["previous"] != "v0" {
		t.Fatalf("web state: %d %v", code, body)
	}
	if code, _ := getJSON(t, ts.URL+"/stacks/nope", nil); code != 404 {
		t.Fatalf("unknown stack should 404, got %d", code)
	}
}

func TestDeployAuth(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	// token configured: no/wrong bearer → 401
	if code, _ := postJSON(t, ts.URL+"/deploy", `{"stack":"web","version":"v2"}`, nil); code != 401 {
		t.Fatalf("no bearer → 401, got %d", code)
	}
	if code, _ := postJSON(t, ts.URL+"/deploy", `{"stack":"web","version":"v2"}`, map[string]string{"Authorization": "Bearer wrong"}); code != 401 {
		t.Fatalf("wrong bearer → 401, got %d", code)
	}
}

func TestDeployDisabledWithoutToken(t *testing.T) {
	ts, _, _ := newTestServer(t, func(o *Options) { o.APIToken = "" })
	code, body := postJSON(t, ts.URL+"/deploy", `{"stack":"web","version":"v2"}`, nil)
	if code != 503 {
		t.Fatalf("unset token → 503, got %d %v", code, body)
	}
}

func auth() map[string]string { return map[string]string{"Authorization": "Bearer tok"} }

func TestDeployHappyPath(t *testing.T) {
	ts, fake, _ := newTestServer(t, nil)
	code, body := postJSON(t, ts.URL+"/deploy", `{"stack":"web","version":"v2"}`, auth())
	if code != 202 || body["id"] == nil {
		t.Fatalf("deploy: %d %v", code, body)
	}
	job := waitJob(t, ts.URL, fmt.Sprint(body["id"]))
	if job["status"] != "done" {
		t.Fatalf("job should succeed: %v", job)
	}
	deploys, _ := fake.snapshot()
	if len(deploys) != 1 || deploys[0] != "web:v2" {
		t.Fatalf("engine calls: %v", deploys)
	}
}

func TestDeployValidation(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	if code, _ := postJSON(t, ts.URL+"/deploy", `{"stack":"nope","version":"v2"}`, auth()); code != 404 {
		t.Fatalf("unknown stack → 404, got %d", code)
	}
	if code, _ := postJSON(t, ts.URL+"/deploy", `{"stack":"web"}`, auth()); code != 400 {
		t.Fatalf("missing version → 400, got %d", code)
	}
	if code, _ := postJSON(t, ts.URL+"/deploy", `not json`, auth()); code != 400 {
		t.Fatalf("bad body → 400, got %d", code)
	}
}

func TestDeployRecordsFailure(t *testing.T) {
	ts, fake, _ := newTestServer(t, nil)
	fake.failWith(fmt.Errorf("deploy exploded"))
	code, body := postJSON(t, ts.URL+"/deploy", `{"stack":"web","version":"v2"}`, auth())
	if code != 202 {
		t.Fatalf("enqueue: %d %v", code, body)
	}
	job := waitJob(t, ts.URL, fmt.Sprint(body["id"]))
	if job["status"] != "failed" || job["error"] != "deploy exploded" {
		t.Fatalf("failure should be recorded: %v", job)
	}
}

func TestDeployConflictWhenBusy(t *testing.T) {
	ts, fake, _ := newTestServer(t, nil)
	fake.mu.Lock()
	fake.block = make(chan struct{})
	fake.mu.Unlock()

	code, firstBody := postJSON(t, ts.URL+"/deploy", `{"stack":"web","version":"v2"}`, auth())
	if code != 202 {
		t.Fatalf("first deploy: %d %v", code, firstBody)
	}
	// wait until the engine is inside Deploy (blocked)
	time.Sleep(50 * time.Millisecond)

	code, conflict := postJSON(t, ts.URL+"/deploy", `{"stack":"web","version":"v3"}`, auth())
	if code != 409 {
		t.Fatalf("busy stack → 409, got %d %v", code, conflict)
	}

	fake.mu.Lock()
	close(fake.block)
	fake.mu.Unlock()
	if job := waitJob(t, ts.URL, fmt.Sprint(firstBody["id"])); job["status"] != "done" {
		t.Fatalf("first job should finish: %v", job)
	}
}

func TestRollbackHappyPath(t *testing.T) {
	ts, fake, _ := newTestServer(t, nil)
	code, body := postJSON(t, ts.URL+"/rollback", `{"stack":"web"}`, auth())
	if code != 202 {
		t.Fatalf("rollback: %d %v", code, body)
	}
	waitJob(t, ts.URL, fmt.Sprint(body["id"]))
	_, rollbacks := fake.snapshot()
	if len(rollbacks) != 1 || rollbacks[0] != "web" {
		t.Fatalf("rollback calls: %v", rollbacks)
	}
}

func ghSig(body string) string {
	m := hmac.New(sha256.New, []byte("hooksecret"))
	m.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(m.Sum(nil))
}

func postWebhook(t *testing.T, url, event, body string, sigHeader string) int {
	t.Helper()
	req, _ := http.NewRequest(http.MethodPost, url, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-GitHub-Event", event)
	if sigHeader != "" {
		req.Header.Set("X-Hub-Signature-256", sigHeader)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	return resp.StatusCode
}

func TestWebhookRejectsBadSignature(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	body := `{"ref":"refs/tags/v1.0.0","after":"x","repository":{"full_name":"oti-adjei/rue"}}`
	if code := postWebhook(t, ts.URL+"/hooks/github", "push", body, ghSig("wrong")); code != 401 {
		t.Fatalf("bad signature → 401, got %d", code)
	}
	if code := postWebhook(t, ts.URL+"/hooks/github", "push", body, ""); code != 401 {
		t.Fatalf("missing signature → 401, got %d", code)
	}
}

func TestWebhookRejectsAllWhenSecretUnset(t *testing.T) {
	ts, _, _ := newTestServer(t, func(o *Options) { o.WebhookSecret = nil })
	body := `{}`
	if code := postWebhook(t, ts.URL+"/hooks/github", "push", body, ghSig("hooksecret")); code != 401 {
		t.Fatalf("unset secret → 401, got %d", code)
	}
}

func TestWebhookPing(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	if code := postWebhook(t, ts.URL+"/hooks/github", "ping", `{}`, ghSig("{}")); code != 200 {
		t.Fatalf("ping → 200, got %d", code)
	}
}

func TestWebhookPushDeploysMatchingTag(t *testing.T) {
	ts, fake, _ := newTestServer(t, nil)
	body := `{"ref":"refs/tags/v1.2.3","after":"deadbeef","repository":{"full_name":"oti-adjei/rue"}}`
	code, resp := postJSON(t, ts.URL+"/hooks/github", body, map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": ghSig(body)})
	if code != 202 {
		t.Fatalf("matching push → 202, got %d %v", code, resp)
	}
	waitJob(t, ts.URL, fmt.Sprint(resp["id"]))
	deploys, _ := fake.snapshot()
	if len(deploys) != 1 || deploys[0] != "rue:v1.2.3" {
		t.Fatalf("tag deploy: %v", deploys)
	}
}

func TestWebhookPushIgnoresNonMatching(t *testing.T) {
	ts, fake, _ := newTestServer(t, nil)
	// wrong repo
	body := `{"ref":"refs/tags/v9.9.9","after":"x","repository":{"full_name":"someone/else"}}`
	if code, resp := postJSON(t, ts.URL+"/hooks/github", body, map[string]string{"X-GitHub-Event": "push", "X-Hub-Signature-256": ghSig(body)}); code != 200 || resp["status"] != "ignored" {
		t.Fatalf("non-matching push → 200 ignored, got %d %v", code, resp)
	}
	// non-push event
	if code := postWebhook(t, ts.URL+"/hooks/github", "issues", `{}`, ghSig("{}")); code != 200 {
		t.Fatalf("other event → 200, got %d", code)
	}
	deploys, _ := fake.snapshot()
	if len(deploys) != 0 {
		t.Fatalf("nothing should deploy: %v", deploys)
	}
}

func TestJobsUnknownID(t *testing.T) {
	ts, _, _ := newTestServer(t, nil)
	if code, _ := getJSON(t, ts.URL+"/jobs/zzz", nil); code != 404 {
		t.Fatalf("unknown job → 404, got %d", code)
	}
}
