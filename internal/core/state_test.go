package core

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := State{Current: "v2", Previous: "v1"}
	if err := SaveState(dir, in); err != nil {
		t.Fatal(err)
	}
	got, err := LoadState(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.Current != "v2" || got.Previous != "v1" {
		t.Fatalf("got %+v", got)
	}
	if _, err := filepath.Glob(filepath.Join(dir, ".zook", "state.json")); err != nil {
		t.Fatal(err)
	}
}

func TestLoadMissingIsZero(t *testing.T) {
	got, err := LoadState(t.TempDir())
	if err != nil {
		t.Fatalf("missing state should not error: %v", err)
	}
	if got.Current != "" || len(got.History) != 0 {
		t.Fatalf("want zero state, got %+v", got)
	}
}

func TestRecordSuccessRotatesPrevious(t *testing.T) {
	now := time.Date(2026, 8, 13, 12, 0, 0, 0, time.UTC)
	s := State{Current: "v1"}
	s.recordSuccess("v2", now)
	if s.Current != "v2" || s.Previous != "v1" {
		t.Fatalf("got current=%s previous=%s", s.Current, s.Previous)
	}
	if len(s.History) != 1 || s.History[0].Result != ResultSuccess || s.History[0].Version != "v2" {
		t.Fatalf("bad history: %+v", s.History)
	}
}

func TestRecordRolledBackAndFailedAppendOnly(t *testing.T) {
	now := time.Now()
	s := State{Current: "v1", Previous: "v0"}
	s.recordRolledBack("v2", now)
	s.recordFailed("v3", now)
	if s.Current != "v1" || s.Previous != "v0" {
		t.Fatalf("current/previous must be untouched, got %+v", s)
	}
	if s.History[0].Result != ResultRolledBack || s.History[1].Result != ResultFailed {
		t.Fatalf("bad history: %+v", s.History)
	}
}
