// Package core holds Zook's runtime-agnostic release engine and state.
package core

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// Result is the outcome recorded for a deploy or rollback attempt.
type Result string

const (
	ResultSuccess    Result = "success"
	ResultRolledBack Result = "rolled_back"
	ResultFailed     Result = "failed"
)

// HistoryEntry is one row of a stack's release history.
type HistoryEntry struct {
	Version   string    `json:"version"`
	Timestamp time.Time `json:"timestamp"`
	Result    Result    `json:"result"`
}

// State is the persisted release state for a single stack.
type State struct {
	Current  string         `json:"current"`
	Previous string         `json:"previous"`
	History  []HistoryEntry `json:"history"`
}

func zookDir(stackDir string) string   { return filepath.Join(stackDir, ".zook") }
func statePath(stackDir string) string { return filepath.Join(zookDir(stackDir), "state.json") }

// LoadState reads a stack's state; a missing file yields a zero State.
func LoadState(stackDir string) (State, error) {
	data, err := os.ReadFile(statePath(stackDir))
	if errors.Is(err, fs.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, err
	}
	var s State
	if err := json.Unmarshal(data, &s); err != nil {
		return State{}, err
	}
	return s, nil
}

// SaveState writes state.json atomically (temp file + rename).
func SaveState(stackDir string, s State) error {
	if err := os.MkdirAll(zookDir(stackDir), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(zookDir(stackDir), "state-*.json")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, statePath(stackDir))
}

func (s *State) recordSuccess(version string, now time.Time) {
	if s.Current != "" {
		s.Previous = s.Current
	}
	s.Current = version
	s.History = append(s.History, HistoryEntry{Version: version, Timestamp: now, Result: ResultSuccess})
}

func (s *State) recordRolledBack(version string, now time.Time) {
	s.History = append(s.History, HistoryEntry{Version: version, Timestamp: now, Result: ResultRolledBack})
}

func (s *State) recordFailed(version string, now time.Time) {
	s.History = append(s.History, HistoryEntry{Version: version, Timestamp: now, Result: ResultFailed})
}
