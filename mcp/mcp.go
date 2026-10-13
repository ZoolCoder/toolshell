// Package mcp is a stdio MCP server for a CLI: newline-delimited JSON-RPC 2.0
// on stdin/stdout, where every tool is one of the CLI's own commands run
// in-process. An agent sees what the CLI prints and can do nothing the CLI
// would not do. There is no generic "run" tool, every value is passed as
// -flag=value so none can be read as a flag of its own, and each command runs
// with an empty stdin, so any confirmation prompt fails closed.
//
// A tool marked Plan saves the plan the command writes and answers with a
// plan_id; a tool marked Apply takes only that id. An agent therefore cannot
// apply a plan body it wrote itself.
package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Protocol is the MCP revision the server answers initialize with.
const Protocol = "2025-06-18"

// Prop is one input of a tool and the flag it becomes.
type Prop struct {
	Name, Type, Desc string // Type: string | boolean | number | array
	Flag             string
	Required         bool
}

// Opt is an optional input.
func Opt(name, typ, desc, flag string) Prop { return Prop{name, typ, desc, flag, false} }

// Req is a required input.
func Req(name, typ, desc, flag string) Prop { return Prop{name, typ, desc, flag, true} }

// Tool is one MCP tool backed by a CLI command.
type Tool struct {
	Name, Desc string
	Cmd        []string // the command and any fixed arguments before the flags
	Props      []Prop
	Tail       []string // fixed arguments after the flags, such as -json
	Write      bool     // changes state outside this machine
	Plan       bool     // saves the plan the command writes with -out and returns plan_id
	Apply      bool     // takes plan_id and runs Server.ApplyCmd on the saved plan
}

// Prompts serves skills as MCP prompts; see Skills for an embed.FS of
// SKILL.md files.
type Prompts interface {
	Names() []string
	Description(name string) string
	Text(name string) string
}

// Server answers MCP requests for one CLI.
type Server struct {
	Name, Version, Instructions string
	Tools                       []Tool
	Prompts                     Prompts // optional

	// Run is the CLI itself: the same function main calls.
	Run func(args []string, stdin io.Reader, stdout, stderr io.Writer) error
	// Result reports errors that are an answer rather than a failure, such as
	// an exit code that means "drift found". Optional.
	Result func(error) bool

	// StateDir holds saved plans; required when a tool sets Plan or Apply.
	StateDir string
	// ApplyCmd is the command a saved plan is applied with; default "apply".
	ApplyCmd []string
	// OutFlag is the flag the plan command writes its plan file to; default "out".
	OutFlag string
	// Cancelled maps the error the CLI returns when a confirmation it cannot
	// get stops an apply to the message the agent sees. Optional.
	Cancelled func(err error, args []string) error
}

// ErrNoTool is returned for a tools/call naming a tool the server lacks.
var ErrNoTool = errors.New("unknown tool")

type request struct {
	ID     json.RawMessage `json:"id,omitempty"`
	Method string          `json:"method"`
	Params json.RawMessage `json:"params,omitempty"`
}

// Error is a JSON-RPC error.
type Error struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Response is a JSON-RPC response.
type Response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *Error          `json:"error,omitempty"`
}

// Serve reads requests from in until EOF and writes replies to out.
func (s *Server) Serve(in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 16<<20)
	enc := json.NewEncoder(out)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var req request
		if err := json.Unmarshal(line, &req); err != nil {
			_ = enc.Encode(Response{JSONRPC: "2.0", ID: json.RawMessage("null"), Error: &Error{-32700, "parse error"}})
			continue
		}
		resp, reply := s.handle(req)
		if reply {
			if err := enc.Encode(resp); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}

// handle answers one message; notifications (no id) produce no reply.
func (s *Server) handle(req request) (Response, bool) {
	resp := Response{JSONRPC: "2.0", ID: req.ID}
	if len(req.ID) == 0 || string(req.ID) == "null" {
		return resp, false
	}
	switch req.Method {
	case "initialize":
		caps := map[string]any{"tools": map[string]any{}}
		if s.Prompts != nil {
			caps["prompts"] = map[string]any{}
		}
		result := map[string]any{
			"protocolVersion": Protocol,
			"capabilities":    caps,
			"serverInfo":      map[string]any{"name": s.Name, "version": s.Version},
		}
		if s.Instructions != "" {
			result["instructions"] = s.Instructions
		}
		resp.Result = result
	case "ping":
		resp.Result = map[string]any{}
	case "tools/list":
		list := []map[string]any{}
		for _, t := range s.Tools {
			list = append(list, map[string]any{"name": t.Name, "description": t.Desc,
				"inputSchema": t.schema(), "annotations": t.annotations()})
		}
		resp.Result = map[string]any{"tools": list}
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &params); err != nil {
			resp.Error = &Error{-32602, "invalid params"}
			break
		}
		text, err := s.Call(params.Name, params.Arguments)
		if errors.Is(err, ErrNoTool) {
			resp.Error = &Error{-32602, err.Error()}
			break
		}
		result := map[string]any{"content": []map[string]any{{"type": "text", "text": text}}}
		if err != nil {
			result["content"] = []map[string]any{{"type": "text", "text": strings.TrimSpace(text + "\nerror: " + err.Error())}}
			result["isError"] = true
		}
		resp.Result = result
	case "prompts/list":
		list := []map[string]any{}
		if s.Prompts != nil {
			for _, name := range s.Prompts.Names() {
				list = append(list, map[string]any{"name": name, "description": s.Prompts.Description(name)})
			}
		}
		resp.Result = map[string]any{"prompts": list}
	case "prompts/get":
		var params struct {
			Name string `json:"name"`
		}
		_ = json.Unmarshal(req.Params, &params)
		text := ""
		if s.Prompts != nil {
			text = s.Prompts.Text(params.Name)
		}
		if text == "" {
			resp.Error = &Error{-32602, "unknown prompt: " + params.Name}
			break
		}
		resp.Result = map[string]any{
			"description": s.Prompts.Description(params.Name),
			"messages":    []map[string]any{{"role": "user", "content": map[string]any{"type": "text", "text": text}}},
		}
	default:
		resp.Error = &Error{-32601, "method not found: " + req.Method}
	}
	return resp, true
}

// Call runs one tool with the agent's arguments and returns its output.
func (s *Server) Call(name string, in map[string]any) (string, error) {
	for _, t := range s.Tools {
		if t.Name != name {
			continue
		}
		if t.Apply {
			return s.applyPlan(scalar(in["plan_id"]))
		}
		args, err := t.Args(in)
		if err != nil {
			return "", err
		}
		if t.Plan {
			return s.savePlan(args)
		}
		return s.Exec(args)
	}
	return "", fmt.Errorf("%w: %s", ErrNoTool, name)
}

// Exec runs one command with an empty stdin and returns what it printed:
// stdout, then stderr.
func (s *Server) Exec(args []string) (string, error) {
	var stdout, stderr bytes.Buffer
	err := s.Run(args, strings.NewReader(""), &stdout, &stderr)
	text := strings.TrimSpace(stdout.String())
	if e := strings.TrimSpace(stderr.String()); e != "" {
		text = strings.TrimSpace(text + "\n\n" + e)
	}
	if err != nil && s.Result != nil && s.Result(err) {
		err = nil
	}
	return text, err
}

func (t Tool) schema() map[string]any {
	props := map[string]any{}
	var required []string
	for _, pr := range t.Props {
		s := map[string]any{"type": pr.Type}
		if pr.Desc != "" {
			s["description"] = pr.Desc
		}
		if pr.Type == "array" {
			s["items"] = map[string]any{"type": "string"}
		}
		props[pr.Name] = s
		if pr.Required {
			required = append(required, pr.Name)
		}
	}
	schema := map[string]any{"type": "object", "properties": props}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func (t Tool) annotations() map[string]any {
	return map[string]any{
		"readOnlyHint":    !t.Write,
		"destructiveHint": t.Write,
		"idempotentHint":  !t.Write,
		"openWorldHint":   true,
	}
}

// Args turns the tool input into a command line: the command, then flags,
// then the fixed tail.
func (t Tool) Args(in map[string]any) ([]string, error) {
	args := append([]string{}, t.Cmd...)
	for _, pr := range t.Props {
		v, ok := in[pr.Name]
		if !ok || v == nil {
			if pr.Required {
				return nil, fmt.Errorf("%s is required", pr.Name)
			}
			continue
		}
		switch pr.Type {
		case "boolean":
			if b, _ := v.(bool); b {
				args = append(args, "-"+pr.Flag)
			}
		case "array":
			for _, s := range anyStrings(v) {
				args = append(args, "-"+pr.Flag+"="+s)
			}
		default:
			s := scalar(v)
			if pr.Required && s == "" {
				return nil, fmt.Errorf("%s is required", pr.Name)
			}
			args = append(args, "-"+pr.Flag+"="+s)
		}
	}
	return append(args, t.Tail...), nil
}

func scalar(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprint(int64(x))
		}
		return fmt.Sprint(x)
	case bool:
		return fmt.Sprint(x)
	}
	b, _ := json.Marshal(v)
	return string(b)
}

func anyStrings(v any) []string {
	var out []string
	switch list := v.(type) {
	case []any:
		for _, e := range list {
			out = append(out, scalar(e))
		}
	case string:
		out = append(out, list)
	}
	return out
}
