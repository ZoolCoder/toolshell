package mcp

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

var errDrift = errors.New("drift")

// fakeCLI records each call and plays a tiny plan/apply tool: plan writes
// its flags to -out, apply reads the plan file it is given.
type fakeCLI struct{ calls [][]string }

func (f *fakeCLI) run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	f.calls = append(f.calls, args)
	switch args[0] {
	case "status":
		_, _ = fmt.Fprintln(stdout, `{"drift":true}`)
		_, _ = fmt.Fprintln(stderr, "1 app drifted")
		return errDrift
	case "plan":
		fs := flag.NewFlagSet("plan", flag.ContinueOnError)
		app := fs.String("app", "", "")
		out := fs.String("out", "", "")
		fs.Bool("json", false, "")
		fs.Bool("confirm", false, "")
		if err := fs.Parse(args[1:]); err != nil {
			return err
		}
		if err := os.WriteFile(*out, []byte("plan for "+*app), 0o600); err != nil {
			return err
		}
		_, _ = fmt.Fprintf(stdout, `{"app":%q}`, *app)
		return nil
	case "apply":
		b, err := os.ReadFile(args[len(args)-1])
		if err != nil {
			return err
		}
		if b, _ := io.ReadAll(stdin); len(b) == 0 && slices.Contains(args, "-confirm") {
			return errors.New("cancelled")
		}
		_, _ = fmt.Fprintf(stdout, "applied %s with %v", b, args[1:len(args)-1])
		return nil
	}
	return errors.New("unknown command")
}

func newServer(t *testing.T) (*Server, *fakeCLI) {
	t.Helper()
	cli := &fakeCLI{}
	return &Server{
		Name: "demo", Version: "1.0.0", Instructions: "plan first",
		Tools: []Tool{
			{Name: "status", Desc: "drift", Cmd: []string{"status"},
				Props: []Prop{Opt("apps", "array", "", "app")}, Tail: []string{"-json"}},
			{Name: "plan", Desc: "plan", Cmd: []string{"plan"}, Plan: true,
				Props: []Prop{Req("app", "string", "app name", "app"), Opt("confirm", "boolean", "", "confirm")},
				Tail:  []string{"-json"}},
			{Name: "apply", Desc: "apply", Apply: true, Write: true,
				Props: []Prop{Req("plan_id", "string", "", "")}},
		},
		Prompts:  Skills(fstest.MapFS{"plan-apply/SKILL.md": {Data: []byte("---\nname: plan-apply\ndescription: Plan then apply.\n---\nbody")}}),
		Run:      cli.run,
		Result:   func(err error) bool { return errors.Is(err, errDrift) },
		StateDir: t.TempDir(),
		Cancelled: func(err error, args []string) error {
			return fmt.Errorf("needs a person: run %s", strings.Join(args, " "))
		},
	}, cli
}

func session(t *testing.T, s *Server, lines ...string) []Response {
	t.Helper()
	var out bytes.Buffer
	if err := s.Serve(strings.NewReader(strings.Join(lines, "\n")), &out); err != nil {
		t.Fatal(err)
	}
	var replies []Response
	dec := json.NewDecoder(&out)
	for dec.More() {
		var r Response
		if err := dec.Decode(&r); err != nil {
			t.Fatal(err)
		}
		replies = append(replies, r)
	}
	return replies
}

func TestServeAnswersEveryMethod(t *testing.T) {
	s, _ := newServer(t)
	replies := session(t, s,
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized"}`,
		`{"jsonrpc":"2.0","id":2,"method":"tools/list"}`,
		`{"jsonrpc":"2.0","id":3,"method":"prompts/list"}`,
		`{"jsonrpc":"2.0","id":4,"method":"prompts/get","params":{"name":"plan-apply"}}`,
		`{"jsonrpc":"2.0","id":5,"method":"prompts/get","params":{"name":"../x"}}`,
		`{"jsonrpc":"2.0","id":6,"method":"nope"}`,
		`not json`,
		`{"jsonrpc":"2.0","id":7,"method":"tools/call","params":{"name":"missing"}}`)
	if len(replies) != 8 {
		t.Fatalf("want 8 replies (notification skipped), got %d", len(replies))
	}
	b, _ := json.Marshal(replies)
	got := string(b)
	for _, want := range []string{`"protocolVersion":"2025-06-18"`, `"instructions":"plan first"`,
		`"readOnlyHint":false`, `"required":["app"]`, `"description":"Plan then apply."`, `"text":"---`} {
		if !strings.Contains(got, want) {
			t.Errorf("replies lack %s", want)
		}
	}
	codes := map[int]bool{}
	for _, r := range replies {
		if r.Error != nil {
			codes[r.Error.Code] = true
		}
	}
	for _, c := range []int{-32602, -32601, -32700} {
		if !codes[c] {
			t.Errorf("no error %d in %s", c, got)
		}
	}
}

func TestCallPassesValuesAsFlagsAndTreatsResultErrorsAsAnswers(t *testing.T) {
	s, cli := newServer(t)
	text, err := s.Call("status", map[string]any{"apps": []any{"web", "-delete"}})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"status", "-app=web", "-app=-delete", "-json"}
	if !slices.Equal(cli.calls[0], want) {
		t.Fatalf("args %v, want %v", cli.calls[0], want)
	}
	if !strings.Contains(text, `{"drift":true}`) || !strings.Contains(text, "1 app drifted") {
		t.Fatalf("text %q", text)
	}
	if _, err := s.Call("plan", map[string]any{}); err == nil || !strings.Contains(err.Error(), "app is required") {
		t.Fatalf("missing required: %v", err)
	}
}

func TestPlanThenApplyByID(t *testing.T) {
	s, cli := newServer(t)
	text, err := s.Call("plan", map[string]any{"app": "web"})
	if err != nil {
		t.Fatal(err)
	}
	var reply struct {
		ID   string         `json:"plan_id"`
		Plan map[string]any `json:"plan"`
	}
	if err := json.Unmarshal([]byte(text), &reply); err != nil || reply.Plan["app"] != "web" {
		t.Fatalf("plan reply %q: %v", text, err)
	}
	info, err := os.Stat(filepath.Join(s.StateDir, "plans", reply.ID+".plan"))
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("saved plan: %v %v", info, err)
	}

	text, err = s.Call("apply", map[string]any{"plan_id": reply.ID})
	if err != nil {
		t.Fatal(err)
	}
	if text != "applied plan for web with [-app=web]" {
		t.Fatalf("apply text %q", text)
	}
	last := cli.calls[len(cli.calls)-1]
	if last[0] != "apply" || slices.Contains(last, "-json") {
		t.Fatalf("apply args %v", last)
	}

	for _, id := range []string{"../../etc/passwd", strings.Repeat("A", 16), "0000000000000000"} {
		if _, err := s.Call("apply", map[string]any{"plan_id": id}); err == nil {
			t.Errorf("plan_id %q accepted", id)
		}
	}
}

func TestApplyCancelledByConfirmation(t *testing.T) {
	s, _ := newServer(t)
	text, err := s.Call("plan", map[string]any{"app": "web", "confirm": true})
	if err != nil {
		t.Fatal(err)
	}
	var reply struct {
		ID string `json:"plan_id"`
	}
	_ = json.Unmarshal([]byte(text), &reply)
	_, err = s.Call("apply", map[string]any{"plan_id": reply.ID})
	if err == nil || !strings.Contains(err.Error(), "needs a person: run apply -app=web -confirm") {
		t.Fatalf("cancelled apply: %v", err)
	}
}

func TestPlanNeedsStateDir(t *testing.T) {
	s, _ := newServer(t)
	s.StateDir = ""
	if _, err := s.Call("plan", map[string]any{"app": "web"}); err == nil {
		t.Fatal("plan without StateDir succeeded")
	}
}

func TestStateDir(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/state")
	if got := StateDir("demo"); got != "/state/demo" {
		t.Fatalf("StateDir %q", got)
	}
}
