# Changelog

All notable changes to toolshell. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and versions follow
[Semantic Versioning](https://semver.org/).

## v0.3.0 — 2026-10-13

### Added

- `guard`: a loopback-only listen check, a handler that refuses a foreign
  `Host` and cross-site writes, sets the headers and caps the body, and an
  `http.Server` with timeouts and graceful shutdown.
- `mcp`: a stdio MCP server that maps tools to the CLI's own commands, serves
  skills as prompts, and applies only plans it saved itself, by id.
- `doctor`: the ok, warn and fail check report, as text or JSON.

## v0.2.0 — 2026-10-12

### Added

- Translations. Pages call `{{.T "text"}}`, or `{{.TH ...}}` for a sentence
  that carries markup. Apps supply `Config.Messages`, and the shell ships
  Arabic for its own text.
- Right-to-left layout for any `Lang` with `Dir: "rtl"`. Icons wrapped in
  `.flip` or `.back` mirror only where they need to.
- A language menu and `GET /lang`. The choice is kept in the
  `<cookie>-lang` cookie.
- A profile menu that shows `Config.User`, the app's `Config.Menu` links,
  the theme switch and the keyboard shortcuts.
- Styled error pages: `ErrorPage`, and `Render`, which turns a template
  error into the 500 page instead of half a page.
- Free text such as provider errors keeps its own direction inside a
  right-to-left page.

### Fixed

- The skip link no longer causes a horizontal scroll in right-to-left pages.

## v0.1.0 — 2026-10-12

First release.

### Added

- The shell: a sidebar that folds to icons and slides in on phones, a
  header, a Ctrl/Cmd-K command palette and `g` then key jumps.
- Light, dark and auto themes, kept in a cookie the server reads.
- Toasts, tooltips, confirm dialogs, and a blocking veil while a long form
  runs.
- Sortable, searchable and paged tables, with CSV export guarded against
  spreadsheet formulas.
- SVG chart template functions: `spark`, `donut`, `cols`, `ring`, `meter`,
  `bars` and `ints`.
- A strict Content-Security-Policy through `SetHeaders`, with every asset
  served from `/_ts/`.
