// Package toolshell is the ZoolCoder app shell for server-rendered Go apps:
// a sidebar that folds to icons and slides in on phones, a header with a
// profile menu and a language switch (right-to-left included), a Ctrl/Cmd-K
// command palette, keyboard jumps, tooltips, toasts, a blocking veil while a
// long form runs, confirm dialogs, styled error pages, light/dark/auto
// themes, sortable and searchable tables, SVG charts, and a strict
// Content-Security-Policy. An app brings its
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
	"bytes"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"strings"
)

//go:embed templates/*.html static/*.css static/*.js static/*.svg
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

// Lang is one choice in the language menu. Dir is "rtl" for a script
// written right to left.
type Lang struct {
	Code, Name, Dir string
}

// User is who the profile menu names. A local tool with no login can name
// the machine's user.
type User struct {
	Name, Detail string
}

// Initials are the avatar's letters: the first of each of up to two words.
func (u User) Initials() string {
	var out []rune
	for _, w := range strings.FieldsFunc(u.Name, func(r rune) bool { return r == ' ' || r == '.' || r == '_' || r == '-' || r == '@' }) {
		out = append(out, []rune(strings.ToUpper(w))[0])
		if len(out) == 2 {
			break
		}
	}
	return string(out)
}

// Config is what an app tells the shell once.
type Config struct {
	Brand  Brand
	Status Status
	User   User
	// Menu is the profile menu's links above the shell's own entries.
	Menu []NavItem
	// Langs are the language menu's choices, the default first. With fewer
	// than two the menu is not drawn.
	Langs []Lang
	// Messages translate the app's text: language code, then the English
	// text, then its translation. Pages call {{.T "Run plan"}}. The shell
	// brings its own for the languages it knows; the app's win.
	Messages map[string]map[string]string
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
	cfg  Config
	msgs map[string]map[string]string
}

// New fills defaults and returns the shell.
func New(cfg Config) *Shell {
	if cfg.Cookie == "" {
		cfg.Cookie = "ts"
	}
	if cfg.Brand.Home == "" {
		cfg.Brand.Home = "/"
	}
	if len(cfg.Langs) == 0 {
		cfg.Langs = []Lang{{Code: "en", Name: "English"}}
	}
	k := &Shell{cfg: cfg, msgs: map[string]map[string]string{}}
	for _, src := range []map[string]map[string]string{shellMessages, cfg.Messages} {
		for lang, m := range src {
			if k.msgs[lang] == nil {
				k.msgs[lang] = map[string]string{}
			}
			for en, tr := range m {
				k.msgs[lang][en] = tr
			}
		}
	}
	return k
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
	Lang     Lang   // the language this page is in
	Langs    []Lang // the language menu; drawn when there are two or more
	User     User
	Menu     []NavItem
	msgs     map[string]string
}

// T translates English text into the page's language, then formats it with
// args when there are any. Text with no translation stays English.
func (c Chrome) T(text string, args ...any) string {
	if tr, ok := c.msgs[text]; ok && tr != "" {
		text = tr
	}
	if len(args) > 0 {
		return fmt.Sprintf(text, args...)
	}
	return text
}

// TH is T for a sentence that carries markup: the translation is escaped,
// string arguments are escaped, and only template.HTML arguments, such as
// what code and strong return, go in as markup.
// {{$.TH "Run %s in a terminal" (code "mailctl apply")}}
func (c Chrome) TH(text string, args ...any) template.HTML {
	safe := make([]any, len(args))
	for i, a := range args {
		switch v := a.(type) {
		case template.HTML:
			safe[i] = string(v)
		case string:
			safe[i] = template.HTMLEscapeString(v)
		default:
			safe[i] = template.HTMLEscapeString(fmt.Sprint(v))
		}
	}
	return template.HTML(fmt.Sprintf(template.HTMLEscapeString(c.T(text)), safe...)) // #nosec G203 -- every part is escaped above
}

// ScriptText is the script's own text in the page's language.
func (c Chrome) ScriptText() map[string]string {
	out := make(map[string]string, len(scriptText))
	for _, k := range scriptText {
		out[k] = c.T(k)
	}
	return out
}

// Chrome builds the shell's data for one request.
func (k *Shell) Chrome(r *http.Request, active string, nav []NavGroup) Chrome {
	q := r.URL.Query()
	lang := k.lang(r)
	c := Chrome{
		Brand: k.cfg.Brand, Nav: nav, Active: active, Theme: k.theme(r), Mini: k.mini(r),
		ReturnTo: r.URL.RequestURI(), Msg: q.Get("msg"), Err: q.Get("err"),
		Status: k.cfg.Status, Footer: k.cfg.Footer, Palette: k.cfg.Palette != nil, Cookie: k.cfg.Cookie,
		Lang: lang, Langs: k.cfg.Langs, User: k.cfg.User, Menu: k.cfg.Menu, msgs: k.msgs[lang.Code],
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

// T translates for a request outside a page, such as a flash message a
// form handler sends back.
func (k *Shell) T(r *http.Request, text string, args ...any) string {
	return Chrome{msgs: k.msgs[k.lang(r).Code]}.T(text, args...)
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
		"ring":   Ring,
		"bars":   Bars,
		"meter":  Meter,
		"ints":   Ints,
		"code":   func(s string) template.HTML { return wrap("code", s) },
		"strong": func(s string) template.HTML { return wrap("strong", s) },
		"spark":  Spark,
		"donut":  Donut,
		"cols":   Columns,
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
	mux.HandleFunc("GET /lang", k.setLang)
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

// setLang stores the language and goes back where the user was.
func (k *Shell) setLang(w http.ResponseWriter, r *http.Request) {
	code := k.cfg.Langs[0].Code
	for _, l := range k.cfg.Langs {
		if l.Code == r.URL.Query().Get("set") {
			code = l.Code
		}
	}
	http.SetCookie(w, &http.Cookie{ // nosemgrep: cookie-missing-secure
		Name: k.cfg.Cookie + "-lang", Value: code, Path: "/", MaxAge: 365 * 24 * 3600,
		HttpOnly: true, SameSite: http.SameSiteLaxMode,
	})
	back := SafeNext(r.URL.Query().Get("back"))
	if back == "" {
		back = "/"
	}
	http.Redirect(w, r, back, http.StatusSeeOther)
}

// lang is the cookie's language when the app offers it, else the default.
func (k *Shell) lang(r *http.Request) Lang {
	if c, err := r.Cookie(k.cfg.Cookie + "-lang"); err == nil {
		for _, l := range k.cfg.Langs {
			if l.Code == c.Value {
				return l
			}
		}
	}
	return k.cfg.Langs[0]
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

// wrap escapes s inside one element, for TH's arguments.
func wrap(tag, s string) template.HTML {
	return template.HTML("<" + tag + ">" + template.HTMLEscapeString(s) + "</" + tag + ">") // #nosec G203 -- s is escaped
}

// ErrorPage is the shell's own page for a status the app cannot serve:
// render it with Render(w, t, code, "ts-error", page).
type ErrorPage struct {
	Chrome
	Code            int
	Heading, Detail string
}

// Render runs a template into a buffer before it writes anything, so a
// template that fails is a clean 500 and not half a page.
func Render(w http.ResponseWriter, t *template.Template, status int, name string, data any) {
	var buf bytes.Buffer
	if err := t.ExecuteTemplate(&buf, name, data); err != nil {
		http.Error(w, "the page failed to render: "+err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	_, _ = buf.WriteTo(w)
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
// CSP, no framing, no sniffing, no referrer to other sites, no caching, no
// indexing. The referrer stays same-origin rather than off: with none at
// all, Chrome sends Origin: null on the page's own form posts, which a CSRF
// check cannot tell from a foreign one.
func SetHeaders(h http.Header) {
	h.Set("Content-Security-Policy", ContentSecurityPolicy)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), payment=(), usb=()")
	h.Set("Cross-Origin-Opener-Policy", "same-origin")
	h.Set("Cross-Origin-Resource-Policy", "same-origin")
	h.Set("Cache-Control", "no-store")
	h.Set("X-Robots-Tag", "noindex, nofollow")
}
