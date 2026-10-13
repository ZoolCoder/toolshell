// Package doctor is the report a tool's `doctor` command prints: one line per
// check, ok, warn or fail, as text for people or versioned JSON for agents.
// A warning is something that still works in a reduced way; only a failure
// makes Write return ErrProblems.
package doctor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
)

// SchemaVersion versions the JSON shape, for agents that parse it.
const SchemaVersion = 1

const (
	OK   = "ok"
	Warn = "warn"
	Fail = "fail"
)

// ErrProblems is returned by Write when a check failed.
var ErrProblems = errors.New("doctor found problems")

// Check is one line of the report.
type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

// Report collects checks.
type Report struct {
	SchemaVersion int     `json:"schemaVersion"`
	Version       string  `json:"version"`
	Checks        []Check `json:"checks"`
}

// New starts a report for a tool at version.
func New(version string) *Report {
	return &Report{SchemaVersion: SchemaVersion, Version: version, Checks: []Check{}}
}

// Add appends a check.
func (r *Report) Add(name, status, detail string) {
	r.Checks = append(r.Checks, Check{Name: name, Status: status, Detail: detail})
}

// Err adds an ok check when err is nil and a failing one otherwise.
func (r *Report) Err(name string, err error, okDetail string) {
	if err != nil {
		r.Add(name, Fail, err.Error())
		return
	}
	r.Add(name, OK, okDetail)
}

// Failed reports whether any check failed.
func (r *Report) Failed() bool {
	return slices.ContainsFunc(r.Checks, func(c Check) bool { return c.Status == Fail })
}

// Write prints the report as aligned text, or indented JSON, and returns
// ErrProblems when a check failed.
func (r *Report) Write(w io.Writer, asJSON bool) error {
	if asJSON {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(r); err != nil {
			return err
		}
	} else {
		width := 0
		for _, c := range r.Checks {
			width = max(width, len(c.Name))
		}
		for _, c := range r.Checks {
			if _, err := fmt.Fprintf(w, "%-4s  %-*s  %s\n", c.Status, width, c.Name, c.Detail); err != nil {
				return err
			}
		}
	}
	if r.Failed() {
		return ErrProblems
	}
	return nil
}

// Binary checks the running executable and whether name on PATH is found;
// not on PATH is a warning, since agents and hooks call it by name.
func Binary(name string) Check {
	self, err := os.Executable()
	if err != nil {
		return Check{"binary", Warn, err.Error()}
	}
	onPath, err := exec.LookPath(name)
	if err != nil {
		return Check{"binary", Warn, self + " (" + name + " is not on PATH)"}
	}
	return Check{"binary", OK, self + " (PATH: " + onPath + ")"}
}

// AddCheck appends c.
func (r *Report) AddCheck(c Check) { r.Checks = append(r.Checks, c) }
