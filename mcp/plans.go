package mcp

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// savedFlags is what a Plan tool keeps so the Apply tool repeats the same
// flags without the agent restating them.
type savedFlags struct {
	Flags []string `json:"flags"`
}

var planIDPattern = regexp.MustCompile(`^[0-9a-f]{16}$`)

// StateDir is $XDG_STATE_HOME/<app>, or ~/.local/state/<app>.
func StateDir(app string) string {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		return filepath.Join(dir, app)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), app)
	}
	return filepath.Join(home, ".local", "state", app)
}

func (s *Server) plansDir() (string, error) {
	if s.StateDir == "" {
		return "", errors.New("mcp: Server.StateDir is not set")
	}
	return filepath.Join(s.StateDir, "plans"), nil
}

// savePlan runs the plan with -out to a scratch file, names it by its content
// and flags, and keeps it in the plans directory under that id.
func (s *Server) savePlan(args []string) (string, error) {
	dir, err := s.plansDir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	scratch, err := os.CreateTemp(dir, "plan-*.tmp")
	if err != nil {
		return "", err
	}
	_ = scratch.Close()
	defer func() { _ = os.Remove(scratch.Name()) }()

	out := s.OutFlag
	if out == "" {
		out = "out"
	}
	var stdout, stderr bytes.Buffer
	if err := s.Run(append(args, "-"+out+"="+scratch.Name()), strings.NewReader(""), &stdout, &stderr); err != nil {
		if s.Result == nil || !s.Result(err) {
			return strings.TrimSpace(stderr.String()), err
		}
	}
	saved, err := os.ReadFile(scratch.Name())
	if err != nil {
		return "", err
	}
	sidecar, _ := json.Marshal(savedFlags{Flags: withoutJSON(args[1:])})
	sum := sha256.Sum256(append(saved, sidecar...))
	id := hex.EncodeToString(sum[:8])
	if err := os.WriteFile(filepath.Join(dir, id+".plan"), saved, 0o600); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir, id+".flags.json"), sidecar, 0o600); err != nil {
		return "", err
	}
	plan := bytes.TrimSpace(stdout.Bytes())
	if !json.Valid(plan) {
		plan, _ = json.Marshal(string(plan))
	}
	return fmt.Sprintf("{\"plan_id\": %q,\n\"plan\": %s}", id, plan), nil
}

func (s *Server) applyPlan(id string) (string, error) {
	if !planIDPattern.MatchString(id) {
		return "", fmt.Errorf("plan_id %q is not one the plan tool returned", id)
	}
	dir, err := s.plansDir()
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(filepath.Join(dir, id+".flags.json"))
	if err != nil {
		return "", fmt.Errorf("no saved plan %s: call the plan tool first", id)
	}
	var saved savedFlags
	if err := json.Unmarshal(raw, &saved); err != nil {
		return "", fmt.Errorf("saved plan %s is unreadable: %w", id, err)
	}
	cmd := s.ApplyCmd
	if len(cmd) == 0 {
		cmd = []string{"apply"}
	}
	args := append(append(append([]string{}, cmd...), saved.Flags...), filepath.Join(dir, id+".plan"))
	text, err := s.Exec(args)
	if err != nil && s.Cancelled != nil {
		err = s.Cancelled(err, args)
	}
	return text, err
}

func withoutJSON(args []string) []string {
	var out []string
	for _, a := range args {
		if a != "-json" {
			out = append(out, a)
		}
	}
	return out
}
