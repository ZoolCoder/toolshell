package activity

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndRecentNewestFirst(t *testing.T) {
	l := &Log{Path: filepath.Join(t.TempDir(), "sub", "activity.jsonl")}
	if got, err := l.Recent(10); err != nil || got != nil {
		t.Fatalf("missing file: %v %v", got, err)
	}
	at := time.Date(2026, 10, 13, 9, 0, 0, 0, time.UTC)
	for i, kind := range []string{"plan", "apply", "plan"} {
		if err := l.Append(Entry{At: at.Add(time.Duration(i) * time.Minute), Kind: kind, OK: true}); err != nil {
			t.Fatal(err)
		}
	}
	f, _ := os.OpenFile(l.Path, os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.WriteString("not json\n")
	_ = f.Close()

	got, err := l.Recent(2)
	if err != nil || len(got) != 2 || got[0].Kind != "plan" || got[1].Kind != "apply" {
		t.Fatalf("recent %+v %v", got, err)
	}
	if info, _ := os.Stat(l.Path); info.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", info.Mode())
	}
}

func TestDataDir(t *testing.T) {
	t.Setenv("XDG_DATA_HOME", "/data")
	if got := DataDir("demo"); got != "/data/demo" {
		t.Fatalf("DataDir %q", got)
	}
}
