package toolshell

import (
	"encoding/json"
	"html/template"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

type page struct {
	Chrome
	Body string
}

var nav = []NavGroup{
	{Items: []NavItem{{Href: "/", Label: "Dashboard", Key: "dashboard", Icon: "grid", Hotkey: "d"}}},
	{Title: "Manage", Items: []NavItem{{Href: "/domains", Label: "Domains", Key: "domains", Icon: "globe", Count: 3}}},
}

func newServer(t *testing.T, cfg Config) *httptest.Server {
	t.Helper()
	shell := New(cfg)
	app := fstest.MapFS{"pages/home.html": {Data: []byte(`{{template "ts-top" .}}<p>{{.Body}}</p>{{template "ts-bottom" .}}`)}}
	tmpl, err := shell.Templates(app, "pages/*.html", template.FuncMap{"shout": strings.ToUpper})
	if err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	shell.Mount(mux)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		SetHeaders(w.Header())
		if err := tmpl.ExecuteTemplate(w, "home.html", page{Chrome: shell.Chrome(r, "domains", nav), Body: "hello"}); err != nil {
			t.Error(err)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func get(t *testing.T, url string, cookies ...*http.Cookie) (*http.Response, string) {
	t.Helper()
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	return resp, string(b)
}

func TestShellRendersNavBrandAndPage(t *testing.T) {
	srv := newServer(t, Config{Brand: Brand{Name: "mail", Accent: "ctl", Tagline: "by ZoolCoder"}, Footer: "made <b>here</b>"})
	resp, body := get(t, srv.URL+"/")
	for _, want := range []string{
		`<title>Domains · mailctl</title>`,
		`<p class="navh">Manage</p>`,
		`href="/domains" class="on" aria-current="page"`,
		`<span class="nb">3</span>`,
		`data-hotkey="d"`,
		`<dt><kbd>g</kbd> <kbd>d</kbd></dt><dd>Dashboard</dd>`,
		`<p>hello</p>`,
		`<footer>made <b>here</b></footer>`,
		`<small>by ZoolCoder</small>`,
		`class="mark"`,
		`data-theme="auto"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %s", want)
		}
	}
	if strings.Contains(body, "data-src=") {
		t.Error("without a Palette func the palette should not fetch entries")
	}
	if csp := resp.Header.Get("Content-Security-Policy"); strings.Contains(csp, "unsafe-inline") || !strings.Contains(csp, "script-src 'self'") {
		t.Errorf("CSP %q is not strict", csp)
	}
	if strings.Contains(body, "style=") || strings.Contains(body, "<script>") {
		t.Error("the shell must not carry inline styles or scripts, the CSP blocks them")
	}
}

func TestThemeCookieRoundTrip(t *testing.T) {
	srv := newServer(t, Config{Cookie: "app"})
	resp, _ := get(t, srv.URL+"/theme?set=dark&back=/domains?q=x")
	if loc := resp.Header.Get("Location"); loc != "/domains?q=x" {
		t.Errorf("redirect to %q, want /domains?q=x", loc)
	}
	var theme *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "app-theme" {
			theme = c
		}
	}
	if theme == nil || theme.Value != "dark" || !theme.HttpOnly {
		t.Fatalf("theme cookie = %v, want an HttpOnly app-theme=dark", theme)
	}
	_, body := get(t, srv.URL+"/", theme)
	if !strings.Contains(body, `data-theme="dark"`) {
		t.Error("the shell should render the cookie's theme")
	}

	for _, back := range []string{"//evil.example", "https://evil.example", `/\evil.example`} {
		resp, _ = get(t, srv.URL+"/theme?set=light&back="+back)
		if loc := resp.Header.Get("Location"); loc != "/" {
			t.Errorf("back %q went to %q, want /", back, loc)
		}
	}
	resp, _ = get(t, srv.URL+"/theme?set=purple")
	for _, c := range resp.Cookies() {
		if c.Name == "app-theme" && c.Value != "auto" {
			t.Errorf("an unknown theme stored %q, want auto", c.Value)
		}
	}
}

func TestFoldedSidebarComesFromTheCookie(t *testing.T) {
	srv := newServer(t, Config{Cookie: "app"})
	_, body := get(t, srv.URL+"/")
	if strings.Contains(body, `class="shell mini"`) || !strings.Contains(body, `aria-expanded="true"`) {
		t.Error("the sidebar should start open")
	}
	_, body = get(t, srv.URL+"/", &http.Cookie{Name: "app-side", Value: "mini"})
	if !strings.Contains(body, `class="shell mini"`) || !strings.Contains(body, `aria-label="Expand menu"`) {
		t.Error("the side cookie should fold the sidebar on first paint")
	}
	if !strings.Contains(body, `data-cookie="app"`) {
		t.Error("the script needs the cookie prefix")
	}
}

func TestPaletteServesAppEntries(t *testing.T) {
	srv := newServer(t, Config{Palette: func(*http.Request) []PaletteItem {
		return []PaletteItem{{Label: "Run plan", Post: "/plan/run", Fields: map[string]string{"return_to": "/plan"}}}
	}})
	_, body := get(t, srv.URL+"/")
	if !strings.Contains(body, `data-src="/_ts/palette.json"`) {
		t.Error("the palette should know where its entries are")
	}
	resp, body := get(t, srv.URL+"/_ts/palette.json")
	var items []PaletteItem
	if err := json.Unmarshal([]byte(body), &items); err != nil || len(items) != 1 || items[0].Fields["return_to"] != "/plan" {
		t.Errorf("palette.json = %s (%v)", body, err)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Error("palette entries must not be cached")
	}
}

func TestAssetsAreServed(t *testing.T) {
	srv := newServer(t, Config{})
	for path, typ := range map[string]string{"/_ts/toolshell.css": "text/css", "/_ts/toolshell.js": "javascript", "/_ts/mark.svg": "image/svg+xml"} {
		resp, body := get(t, srv.URL+path)
		if resp.StatusCode != http.StatusOK || body == "" || !strings.Contains(resp.Header.Get("Content-Type"), typ) {
			t.Errorf("%s: status %d, type %q", path, resp.StatusCode, resp.Header.Get("Content-Type"))
		}
	}
}

func TestAppFuncsAndKitIcons(t *testing.T) {
	shell := New(Config{})
	app := fstest.MapFS{"p.html": {Data: []byte(`{{shout "x"}}{{icon "globe"}}{{icon "nope"}}`)}}
	tmpl, err := shell.Templates(app, "*.html", template.FuncMap{"shout": strings.ToUpper})
	if err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	if err := tmpl.ExecuteTemplate(&b, "p.html", nil); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(b.String(), "X<svg") || strings.Count(b.String(), "<svg") != 1 {
		t.Errorf("got %q", b.String())
	}
}

func TestLanguageCookieTurnsThePageRightToLeft(t *testing.T) {
	cfg := Config{
		Brand: Brand{Name: "mail", Accent: "ctl"}, Cookie: "app",
		Langs:    []Lang{{Code: "en", Name: "English"}, {Code: "ar", Name: "العربية", Dir: "rtl"}},
		Messages: map[string]map[string]string{"ar": {"Domains": "النطاقات"}},
	}
	srv := newServer(t, cfg)

	resp, _ := get(t, srv.URL+"/lang?set=ar&back=/")
	if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != "/" {
		t.Fatalf("lang switch = %d to %q", resp.StatusCode, resp.Header.Get("Location"))
	}
	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "app-lang" {
			cookie = c
		}
	}
	if cookie == nil || cookie.Value != "ar" {
		t.Fatalf("no app-lang=ar cookie in %v", resp.Cookies())
	}

	_, body := get(t, srv.URL+"/", cookie)
	for _, want := range []string{`lang="ar" dir="rtl"`, `<title>النطاقات · mailctl</title>`, `القائمة الرئيسية`, `aria-checked="true" href="/lang?set=ar`} {
		if !strings.Contains(body, want) {
			t.Errorf("Arabic page lacks %s", want)
		}
	}
	_, body = get(t, srv.URL+"/", &http.Cookie{Name: "app-lang", Value: "xx"})
	if !strings.Contains(body, `lang="en" dir="ltr"`) {
		t.Error("an unknown language should fall back to the first")
	}
	resp, _ = get(t, srv.URL+"/lang?set=ar&back=//evil.example")
	if loc := resp.Header.Get("Location"); loc != "/" {
		t.Errorf("lang switch followed a foreign back to %q", loc)
	}
}

func TestOneLanguageDrawsNoLanguageMenu(t *testing.T) {
	_, body := get(t, newServer(t, Config{Brand: Brand{Name: "x"}}).URL+"/")
	if strings.Contains(body, `id="ts-langs"`) {
		t.Error("a single language should not draw the language menu")
	}
}

func TestChromeTranslatesAndFormats(t *testing.T) {
	c := Chrome{msgs: map[string]string{"%d domains": "%d نطاقات"}}
	if got := c.T("%d domains", 3); got != "3 نطاقات" {
		t.Errorf("T = %q", got)
	}
	if got := c.T("untranslated"); got != "untranslated" {
		t.Errorf("T = %q", got)
	}
}

func TestUserInitials(t *testing.T) {
	for name, want := range map[string]string{"zoolcoder": "Z", "Ada Lovelace": "AL", "jane.doe@x": "JD", "": ""} {
		if got := (User{Name: name}).Initials(); got != want {
			t.Errorf("Initials(%q) = %q, want %q", name, got, want)
		}
	}
}

func TestProfileMenuNamesTheUser(t *testing.T) {
	srv := newServer(t, Config{Brand: Brand{Name: "x"}, User: User{Name: "zoolcoder", Detail: "127.0.0.1"}, Menu: []NavItem{{Href: "/settings", Label: "Settings", Icon: "gear"}}})
	_, body := get(t, srv.URL+"/")
	for _, want := range []string{`<b>zoolcoder</b><small>127.0.0.1</small>`, `<span class="avatar" aria-hidden="true">Z</span>`, `role="menuitem" href="/settings"`, `id="ts-veil"`, `id="ts-toasts"`, `id="ts-confirm"`} {
		if !strings.Contains(body, want) {
			t.Errorf("page lacks %s", want)
		}
	}
}

func TestRenderErrorPageAndFailedTemplate(t *testing.T) {
	shell := New(Config{Brand: Brand{Name: "x"}})
	tmpl, err := shell.Templates(fstest.MapFS{"p/bad.html": {Data: []byte(`{{.Nope}}`)}}, "p/*.html", nil)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/missing", nil)
	w := httptest.NewRecorder()
	Render(w, tmpl, http.StatusNotFound, "ts-error", ErrorPage{Chrome: shell.Chrome(r, "", nil), Code: 404, Heading: "Page not found"})
	if w.Code != http.StatusNotFound || !strings.Contains(w.Body.String(), `<div class="errcode" aria-hidden="true">404</div>`) {
		t.Errorf("error page = %d %s", w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	Render(w, tmpl, http.StatusOK, "bad.html", struct{}{})
	if w.Code != http.StatusInternalServerError || strings.Contains(w.Body.String(), "<html") {
		t.Errorf("a failed template should be a clean 500, got %d", w.Code)
	}
}

func TestCharts(t *testing.T) {
	if s := string(Donut("ok bad", []int{3, 1})); strings.Count(s, `class="seg `) != 2 || !strings.Contains(s, `class="seg bad"`) {
		t.Errorf("donut = %s", s)
	}
	if s := string(Donut("ok", nil)); strings.Contains(s, "seg") {
		t.Errorf("empty donut drew a segment: %s", s)
	}
	if s := string(Spark([]int{1, 4, 2})); !strings.Contains(s, `class="ln"`) {
		t.Errorf("spark = %s", s)
	}
	s := string(Columns("ok bad", []Column{{Label: "<x>", Values: []int{2, 1}}, {Label: "b", Values: []int{0, 0}}}))
	if strings.Count(s, "<rect") != 2 || !strings.Contains(s, "&lt;x&gt;") {
		t.Errorf("columns = %s", s)
	}
}

func TestTHEscapesAllButMarkupArguments(t *testing.T) {
	c := Chrome{msgs: map[string]string{"Run %s on <%s>": "شغّل %s على <%s>"}}
	got := string(c.TH("Run %s on <%s>", wrap("code", "a<b"), "x&y"))
	if want := "شغّل <code>a&lt;b</code> على &lt;x&amp;y&gt;"; got != want {
		t.Errorf("TH = %q, want %q", got, want)
	}
}
