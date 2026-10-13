package doctor

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func TestWriteText(t *testing.T) {
	r := New("1.2.3")
	r.Add("config", OK, "demo.yaml")
	r.Add("token", Warn, "not set")
	var out bytes.Buffer
	if err := r.Write(&out, false); err != nil {
		t.Fatal(err)
	}
	want := "ok    config  demo.yaml\nwarn  token   not set\n"
	if out.String() != want {
		t.Fatalf("got %q, want %q", out.String(), want)
	}
}

func TestWriteJSONAndFailure(t *testing.T) {
	r := New("1.2.3")
	r.Err("api", errors.New("401"), "")
	r.Err("config", nil, "fine")
	r.AddCheck(Binary("definitely-not-on-path-xyz"))
	var out bytes.Buffer
	if err := r.Write(&out, true); !errors.Is(err, ErrProblems) {
		t.Fatalf("err %v", err)
	}
	var got Report
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != SchemaVersion || got.Version != "1.2.3" || len(got.Checks) != 3 {
		t.Fatalf("report %+v", got)
	}
	if got.Checks[0].Status != Fail || got.Checks[1].Status != OK || got.Checks[2].Status != Warn ||
		!strings.Contains(got.Checks[2].Detail, "not on PATH") {
		t.Fatalf("checks %+v", got.Checks)
	}
}
