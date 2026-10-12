// Package toolshell is the ZoolCoder app shell for server-rendered Go apps:
// a sidebar that folds to icons and slides in on phones, a Ctrl/Cmd-K command
// palette, keyboard jumps, tooltips, light/dark/auto themes, sortable and
// searchable tables, and a strict Content-Security-Policy. An app brings its
// pages and its nav; the shell, the stylesheet and the one script come from
// here.
//
// Every asset is a file under /_ts/, so the CSP needs no 'unsafe-inline'.
// Theme and sidebar state live in cookies the server reads, so the first
// paint is already right without a script.
//
// Wiring, in short:
//
//	ts := toolshell.New(toolshell.Config{Brand: toolshell.Brand{Name: "mail", Accent: "ctl"}, Cookie: "mailctl"})
//	tmpl, _ := ts.Templates(appFS, "templates/*.html", funcs)
//	ts.Mount(mux)
//	page := myPage{Chrome: ts.Chrome(r, "domains", nav)}
//
// and each page template starts with {{template "ts-top" .}} and ends with
// {{template "ts-bottom" .}}.
package toolshell

import (
	"embed"
	"encoding/json"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates/*.html static/*
var assets embed.FS

// Prefix is where the shell serves its stylesheet, script and mark.
const Prefix = "/_ts"

// Themes are the choices in the sidebar switch; auto follows the OS.
var Themes = []string{"light", "dark", "auto"}

// Brand is the sidebar wordmark: Name then Accent in the accent colour,
// with an optional Tagline under it. Mark replaces the ZoolCoder mark.
type Brand struct {
	Name, Accent, Tagline string
	Mark                  template.HTML
	Home                  string // where the brand links; "/" when empty
}

// NavItem is one sidebar entry. Icon names an entry of Icons. Count shows
// as a badge when non-zero. Hotkey is the second key of a "g then key" jump.
type NavItem struct {
	Href, Label, Key, Icon string
	Count                  int
	Hotkey                 string
}

// NavGroup is a run of entries under an optional heading.
type NavGroup struct {
	Title string
	Items []NavItem
}

// Status is the light at the foot of the sidebar.
type Status struct {
	Label, Detail string
	Tone          string // ok (default), warn or bad
}

// PaletteItem is one palette entry the app adds beyond its pages: a link
// (Href) or a button that posts Fields to Post.
type PaletteItem struct {
	Label  string            `json:"label"`
	Hint   string            `json:"hint,omitempty"`
	Href   string            `json:"href,omitempty"`
	Post   string            `json:"post,omitempty"`
	Fields map[string]string `json:"fields,omitempty"`
}

// Config is what an app tells the shell once.
type Config struct {
	Brand  Brand
	Status Status
	// Footer is trusted markup under every page.
	Footer template.HTML
	// Cookie prefixes the theme and sidebar cookies so two apps on one
	// host keep their own choices. "ts" when empty.
	Cookie string
	// Palette adds entries to the command palette; nil leaves it with the
	// nav pages only.
	Palette func(*http.Request) []PaletteItem
}

// Shell is a configured toolshell.
type Shell struct {
	cfg Config
}

// New fills defaults and returns the shell.
func New(cfg Config) *Shell {
	if cfg.Cookie == "" {
		cfg.Cookie = "ts"
	}
	if cfg.Brand.Home == "" {
		cfg.Brand.Home = "/"
	}
	return &Shell{cfg: cfg}
}

// Chrome is what the shell needs on every page. Page data embeds it.
type Chrome struct {
	Brand    Brand
	Nav      []NavGroup
	Active   string // Key of the current entry
	Title    string // the entry's label; a page may overwrite it
	Theme    string // light, dark or auto
	Mini     bool   // the sidebar is folded to icons
	ReturnTo string // where a form on this page sends the user back
	Msg, Err string // flash from ?msg= and ?err=
	Status   Status
	Footer   template.HTML
	Palette  bool   // the app serves palette entries
	Cookie   string // the cookie prefix, for the script
}

// Chrome builds the shell's data for one request.
func (k *Shell) Chrome(r *http.Request, active string, nav []NavGroup) Chrome {
	q := r.URL.Query()
	c := Chrome{
		Brand: k.cfg.Brand, Nav: nav, Active: active, Theme: k.theme(r), Mini: k.mini(r),
		ReturnTo: r.URL.RequestURI(), Msg: q.Get("msg"), Err: q.Get("err"),
		Status: k.cfg.Status, Footer: k.cfg.Footer, Palette: k.cfg.Palette != nil, Cookie: k.cfg.Cookie,
	}
	for _, g := range nav {
		for _, it := range g.Items {
			if it.Key == active {
				c.Title = it.Label
			}
		}
	}
	return c
}

// Hotkeys are the nav entries that have a "g then key" jump, for the
// shortcut list.
func (c Chrome) Hotkeys() []NavItem {
	var out []NavItem
	for _, g := range c.Nav {
		for _, it := range g.Items {
			if it.Hotkey != "" {
				out = append(out, it)
			}
		}
	}
	return out
}

// Templates parses the shared shell and the app's pages into one set, with
// the shell's helpers and the app's funcs (the app's win on a name clash).
// Pages are addressed by base name: "plan.html".
func (k *Shell) Templates(app fs.FS, pattern string, funcs template.FuncMap) (*template.Template, error) {
	all := template.FuncMap{
		"icon":   Icon,
		"themes": func() []string { return Themes },
	}
	for name, fn := range funcs {
		all[name] = fn
	}
	t, err := template.New("").Funcs(all).ParseFS(assets, "templates/*.html")
	if err != nil {
		return nil, err
	}
	return t.ParseFS(app, pattern)
}

// Mount registers the shell's routes: its assets, the theme switch and, when
// the app has palette entries, the palette's data.
func (k *Shell) Mount(mux *http.ServeMux) {
	static, _ := fs.Sub(assets, "static")
	mux.Handle("GET "+Prefix+"/", http.StripPrefix(Prefix+"/", http.FileServerFS(static)))
	mux.HandleFunc("GET /theme", k.setTheme)
	if k.cfg.Palette != nil {
		mux.HandleFunc("GET "+Prefix+"/palette.json", k.palette)
	}
}

// setTheme stores the choice and goes back where the user was.
func (k *Shell) setTheme(w http.ResponseWriter, r *http.Request) {
	t := r.URL.Query().Get("set")
	if !validTheme(t) {
		t = "auto"
	}
	// Not Secure: these dashboards are plain http on loopback, and the
	// cookie only holds a colour scheme.
	http.SetCookie(w, &http.Cookie{ // nosemgrep: cookie-missing-secure
		Name: k.cfg.Cookie + "-theme", Value: t, Path: "/", MaxAge: 365 * 24 * 3600,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	back := SafeNext(r.URL.Query().Get("back"))
	if back == "" {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

func (k *Shell) palette(w http.ResponseWriter, r *http.Request) {
	items := k.cfg.Palette(r)
	if items == nil {
		items = []PaletteItem{}
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(items)
}

func (k *Shell) theme(r *http.Request) string {
	if c, err := r.Cookie(k.cfg.Cookie + "-theme"); err == nil && validTheme(c.Value) {
		return c.Value
	}
	return "auto"
}

// mini reads the sidebar cookie the script sets when the user folds it.
func (k *Shell) mini(r *http.Request) bool {
	c, err := r.Cookie(k.cfg.Cookie + "-side")
	return err == nil && c.Value == "mini"
}

func validTheme(t string) bool {
	for _, v := range Themes {
		if t == v {
			return true
		}
	}
	return false
}

// SafeNext keeps a redirect on this site: a path, never another origin.
func SafeNext(next string) string {
	if strings.HasPrefix(next, "/") && !strings.HasPrefix(next, "//") && !strings.HasPrefix(next, "/\\") {
		return next
	}
	return ""
}

// ContentSecurityPolicy keeps a page to its own origin. The shell's styles and
// script are files, so nothing inline runs or applies.
const ContentSecurityPolicy = "default-src 'self'; script-src 'self'; style-src 'self'; " +
	"img-src 'self' data:; font-src 'self'; connect-src 'self'; " +
	"form-action 'self'; frame-ancestors 'none'; base-uri 'none'; object-src 'none'"

// SetHeaders sets the strict headers every toolshell page is meant to carry: the
// CSP, no framing, no sniffing, no referrer, no caching, no indexing.
func SetHeaders(h http.Header) {
	h.Set("Content-Security-Policy", ContentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "no-referrer")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Robots-Tag", "noindex, nofollow")
}
