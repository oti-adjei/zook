package health

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/oti-adjei/zook/internal/exec"
)

func TestCheckURLHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()
	p := New(http.DefaultClient, exec.OSRunner{})
	if err := p.Check(context.Background(), Config{URL: srv.URL}); err != nil {
		t.Fatalf("want healthy, got %v", err)
	}
}

func TestCheckURLWrongStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(500)
	}))
	defer srv.Close()
	p := New(http.DefaultClient, exec.OSRunner{})
	if err := p.Check(context.Background(), Config{URL: srv.URL, ExpectedStatus: 200}); err == nil {
		t.Fatal("500 should be unhealthy")
	}
}

func TestCheckCommand(t *testing.T) {
	p := New(http.DefaultClient, exec.OSRunner{})
	if err := p.Check(context.Background(), Config{Command: []string{"true"}}); err != nil {
		t.Fatalf("`true` should be healthy: %v", err)
	}
	if err := p.Check(context.Background(), Config{Command: []string{"false"}}); err == nil {
		t.Fatal("`false` should be unhealthy")
	}
}

// fakeDoer returns a scripted sequence of statuses.
type fakeDoer struct {
	statuses []int
	i        int
	err      error
}

func (f *fakeDoer) Do(*http.Request) (*http.Response, error) {
	if f.err != nil {
		return nil, f.err
	}
	s := f.statuses[f.i]
	if f.i < len(f.statuses)-1 {
		f.i++
	}
	return &http.Response{StatusCode: s, Body: http.NoBody}, nil
}

func TestPollBecomesHealthy(t *testing.T) {
	doer := &fakeDoer{statuses: []int{500, 500, 200}}
	p := New(doer, exec.OSRunner{})
	err := p.Poll(context.Background(), Config{URL: "http://x/h"}, time.Second, time.Millisecond)
	if err != nil {
		t.Fatalf("should become healthy: %v", err)
	}
}

func TestPollTimesOut(t *testing.T) {
	doer := &fakeDoer{err: errors.New("connection refused")}
	p := New(doer, exec.OSRunner{})
	err := p.Poll(context.Background(), Config{URL: "http://x/h"}, 20*time.Millisecond, time.Millisecond)
	if err == nil {
		t.Fatal("never-healthy should time out with error")
	}
}

func TestCheckEmptyConfigErrors(t *testing.T) {
	p := New(http.DefaultClient, exec.OSRunner{})
	err := p.Check(context.Background(), Config{})
	if err == nil {
		t.Fatal("empty config should return an error")
	}
}

func TestPollTimesOutPromptly(t *testing.T) {
	doer := &fakeDoer{err: errors.New("connection refused")}
	p := New(doer, exec.OSRunner{})
	timeout := 50 * time.Millisecond
	interval := 200 * time.Millisecond // interval > timeout: sleep must be bounded
	start := time.Now()
	err := p.Poll(context.Background(), Config{URL: "http://x/h"}, timeout, interval)
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("never-healthy should time out with error")
	}
	// Should finish within ~2x the timeout, not a full interval past it.
	if elapsed > 3*timeout {
		t.Fatalf("Poll took %v, expected < %v (bounded sleep)", elapsed, 3*timeout)
	}
}
