// Package daemon manages the thingd web server as a background process for the
// `thing server` commands. thing is the client and thingd the daemon it controls
// (the docker/dockerd split): this package resolves the thingd binary, launches
// it detached, tracks its runtime state, and stops it. There is a single global
// daemon; its state and log live alongside projects.yaml under the state dir.
package daemon

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/muniere/thing/internal/registry"
)

// Runtime file names within the state directory. thingd.log is the daemon's
// combined stdout+stderr; server.json is the runtime state written at start.
const (
	StateFileName = "server.json"
	LogFileName   = "thingd.log"
)

// State is the persisted runtime state of the thingd daemon. thing pins the port
// when it launches thingd, so it records the pid and port itself and thingd needs
// no change to report them.
type State struct {
	PID       int    `json:"pid"`
	Port      int    `json:"port"`
	URL       string `json:"url"`
	StartedAt string `json:"started_at"`
}

// StatePath returns the path to server.json under the state directory.
func StatePath() (string, error) { return stateFile(StateFileName) }

// LogPath returns the path to thingd.log under the state directory.
func LogPath() (string, error) { return stateFile(LogFileName) }

// stateFile joins name onto the resolved state directory. Tests point the whole
// state directory at a temp dir via THING_DATA_DIR (see registry.StateDir).
func stateFile(name string) (string, error) {
	dir, err := registry.StateDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// readState loads server.json. A missing file yields a zero State and ok=false.
func readState(path string) (st State, ok bool, err error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return State{}, false, nil
		}
		return State{}, false, err
	}
	if err := json.Unmarshal(data, &st); err != nil {
		return State{}, false, fmt.Errorf("parse %s: %w", path, err)
	}
	return st, true, nil
}

// writeState writes server.json atomically (temp file + rename), creating the
// enclosing state directory if needed. It mirrors registry.Save's approach so a
// crash mid-write never leaves a truncated state file.
func writeState(path string, st State) error {
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), StateFileName+".*.tmp")
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
	return os.Rename(tmpName, path)
}

// removeState deletes server.json, treating an already-absent file as success.
func removeState(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
