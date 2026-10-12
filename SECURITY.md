# Security

toolshell renders the pages of admin tools, so a cross-site scripting or open
redirect bug in it is a bug in every app built on it.

## Supported versions

Only the latest release gets fixes.

## Reporting a vulnerability

Please report privately, not in a public issue. Open a
[security advisory](https://github.com/zoolcoder/toolshell/security/advisories/new)
on this repository.

Include what you did, what happened, and what you expected. You will get a
reply within 7 days.

## What toolshell guarantees

- **A strict Content-Security-Policy.** `SetHeaders` allows only the page's
  own origin, with no `'unsafe-inline'` and no `'unsafe-eval'`. Every
  stylesheet, script and image is a file under `/_ts/`.
- **Escaped translations.** `T` returns plain text that the template
  escapes. `TH` escapes the translation and trusts only the arguments the
  page passes.
- **Redirects stay on the site.** The theme and language switches pass
  their `back` target through `SafeNext`, which accepts a local path and
  never another origin.
- **CSV export is safe in a spreadsheet.** A cell that starts with `=`, `+`,
  `-` or `@` is prefixed so that it cannot run as a formula.
- **No half pages.** `Render` executes a template into a buffer first, so a
  template error never sends a partial page.

Securing an app's own pages, such as its login, its CSRF checks and the
data it shows, is up to the app.
