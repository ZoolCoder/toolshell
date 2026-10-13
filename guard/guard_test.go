package guard

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLoopbackAddr(t *testing.T) {
	for addr, ok := range map[string]bool{
		"127.0.0.1:0": true, "localhost:8080": true, "[::1]:9": true,
		"0.0.0.0:80": false, ":80": false, "192.168.1.2:80": false, "nope": false,
	} {
		if err := LoopbackAddr(addr); (err == nil) != ok {
			t.Errorf("LoopbackAddr(%q) = %v", addr, err)
		}
	}
}

func TestHostAllowed(t *testing.T) {
	for host, ok := range map[string]bool{
		"127.0.0.1:4000": true, "localhost:1": true, "LOCALHOST": true, "[::1]:4000": true,
		"127.0.0.2": true, "evil.example:4000": false, "127.0.0.1.nip.io": false, "": false,
	} {
		if got := HostAllowed(host, "127.0.0.1:4000"); got != ok {
			t.Errorf("HostAllowed(%q) = %v", host, got)
		}
	}
}

func TestHandler(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "ok "+r.FormValue("name"))
	})
	h := Handler(next, Options{Host: "127.0.0.1:4000", MaxBody: 64})

	cases := []struct {
		name, method, host, body string
		headers                  map[string]string
		code                     int
	}{
		{"get", "GET", "127.0.0.1:4000", "", nil, 200},
		{"rebound host", "GET", "evil.example", "", nil, 403},
		{"same-origin post", "POST", "127.0.0.1:4000", "name=a", map[string]string{"Sec-Fetch-Site": "same-origin"}, 200},
		{"cross-site post", "POST", "127.0.0.1:4000", "name=a", map[string]string{"Sec-Fetch-Site": "cross-site"}, 403},
		{"same-site post", "POST", "127.0.0.1:4000", "name=a", map[string]string{"Sec-Fetch-Site": "same-site"}, 403},
		{"foreign origin", "POST", "127.0.0.1:4000", "name=a", map[string]string{"Origin": "http://evil.example"}, 403},
		{"own origin", "POST", "127.0.0.1:4000", "name=a", map[string]string{"Origin": "http://127.0.0.1:4000"}, 200},
		{"too large", "POST", "127.0.0.1:4000", "name=" + strings.Repeat("a", 100), nil, 413},
	}
	for _, c := range cases {
		r := httptest.NewRequest(c.method, "/", strings.NewReader(c.body))
		r.Host = c.host
		if c.method == "POST" {
			r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
		for k, v := range c.headers {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != c.code {
			t.Errorf("%s: code %d, want %d", c.name, w.Code, c.code)
		}
		if c.code == 200 && w.Header().Get("Content-Security-Policy") == "" {
			t.Errorf("%s: no CSP", c.name)
		}
	}
}

func TestServer(t *testing.T) {
	srv := Server(http.NotFoundHandler(), 0)
	if srv.WriteTimeout != 3*time.Minute || !srv.DisableGeneralOptionsHandler || srv.ReadHeaderTimeout == 0 {
		t.Fatalf("server %+v", srv)
	}
}

func TestListenAndServeRefusesNonLoopbackAndStops(t *testing.T) {
	if err := ListenAndServe(context.Background(), "0.0.0.0:0", http.NotFoundHandler(), nil); err == nil {
		t.Fatal("0.0.0.0 accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	hosts := make(chan string, 1)
	go func() {
		done <- ListenAndServe(ctx, "127.0.0.1:0", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, "hi")
		}), func(h string) { hosts <- h })
	}()
	host := <-hosts
	resp, err := http.Get("http://" + host + "/")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != 200 || resp.Header.Get("X-Frame-Options") != "DENY" {
		t.Fatalf("status %d headers %v", resp.StatusCode, resp.Header)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}
