package toolshell

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestBackCarriesTheFlashAndStaysOnSite(t *testing.T) {
	cases := []struct {
		returnTo, msg string
		err           error
		want          string
	}{
		{"/apps/web?msg=old", "planned", nil, "/apps/web?msg=planned"},
		{"/apps/web", "", errors.New("boom"), "/apps/web?err=boom"},
		{"https://evil.example/", "ok", nil, "/fallback?msg=ok"},
		{"//evil.example/", "ok", nil, "/fallback?msg=ok"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("POST", "/x", strings.NewReader(url.Values{"return_to": {c.returnTo}}.Encode()))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		w := httptest.NewRecorder()
		Back(w, r, "/fallback", c.msg, c.err)
		if w.Code != http.StatusSeeOther || w.Header().Get("Location") != c.want {
			t.Errorf("return_to %q: %d %q, want %q", c.returnTo, w.Code, w.Header().Get("Location"), c.want)
		}
	}
}
