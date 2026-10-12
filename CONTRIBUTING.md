# Contributing

Thanks for helping. toolshell is small on purpose, so read this before you
open a pull request.

## Ground rules

- **Standard library only.** A new dependency needs a very good reason.
- **No inline script or style.** Every asset is a file under `/_ts/`, so the
  Content-Security-Policy needs no `'unsafe-inline'`. A change that needs a
  looser policy is declined.
- **Server first.** A page must render correctly before the script loads.
  The script only improves it.
- **Both directions.** Check every layout change in English and in Arabic
  (right to left), in both themes, and at phone width (390px).
- **Translations.** Every new string the shell shows goes in `messages.go`
  with its Arabic. The tests check that each one has a translation.

## Development

```sh
make test   # go test ./...
make lint   # gofmt and go vet
make fmt    # gofmt -w .
```

Both `make lint` and `make test` must pass before you open a pull request.

To see a change in a real app, point that app at your checkout with a
`go.work` file:

```sh
go work init . ../toolshell
```

## Commits and pull requests

- Use [Conventional Commits](https://www.conventionalcommits.org/):
  `feat: add a meter legend`, `fix(css): keep the skip link on screen`.
- Keep each pull request to one topic, and rebase it on `main`.
- Add a line to `CHANGELOG.md` under the next version for any change a user
  of the package would notice.
- For a visible change, add a before and after screenshot to the pull
  request.

By contributing, you agree that your work is released under the
[MIT License](LICENSE).
