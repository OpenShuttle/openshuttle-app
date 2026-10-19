# Contributing to OpenShuttle

Thanks for helping! OpenShuttle runs badminton club tournaments offline, on a
single PC, often operated by volunteers in a noisy gym. Every change is judged
by one question: does it make the day easier for the control desk?

## Before you start

- **Read the functional scope**: [docs/specs/scope-v1.md](docs/specs/scope-v1.md).
  It holds the vocabulary (glossary) and the business rules.
- **Read the architecture rules** in [CLAUDE.md](CLAUDE.md) and the decisions
  in [docs/adr](docs/adr). They apply to humans and AI assistants alike.
- **Bugs**: open an issue with the bug template. A tournament file
  (`.openshuttle`) that shows the problem helps a lot; remove real names first.
- **Features**: start with a discussion or a feature request. A short agreement
  on the behaviour saves a rewritten pull request. v1 is offline only: network
  features (v2) are out of scope unless a maintainer says otherwise.
- **Security issues**: never in a public issue, see [SECURITY.md](SECURITY.md).

Issues labelled `good first issue` are a good place to start.

## Development setup

You need Go (version in [src/go.mod](src/go.mod)), Node 24, and the Wails CLI
at the version pinned in `src/go.mod`:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.30
```

On Linux, also: `libgtk-4-dev libwebkitgtk-6.0-dev libsoup-3.0-dev`.

```bash
cd src/apps/desktop/frontend && npm ci
cd src/apps/desktop && wails3 dev            # the app with hot reload
cd src && go test ./...                      # Go tests (build the frontend first)
cd src/apps/desktop/frontend && npm test     # frontend tests
cd src/apps/desktop/frontend && npm run i18n:check
```

After changing a Go service, regenerate and commit the bindings:

```bash
cd src/apps/desktop && wails3 generate bindings -clean=true -ts -i
```

## How the code is organized

- **Go is the source of truth.** Business logic (standings, draws, schedule,
  score validity) lives in Go. The Angular frontend only renders views and
  sends commands.
- `src/internal/engine` is pure (no I/O, no clock) and fully tested;
  `src/internal/tournament` holds the event-sourced tournament state;
  `src/apps/desktop` the Wails services; `src/apps/desktop/frontend` the UI.
- Every change to the tournament is an **event**: undoable, replayed on open.
  Event payloads are stored in user files: never rename their JSON fields.
- Go returns **error codes**, never text. The frontend translates them.
- **Never block the user**: warn, and refuse only what is impossible.

## Making a change

1. Fork the repository and create a branch from `main`.
2. Keep the change focused: one feature or fix per pull request.
3. Write tests: every engine or tournament change comes with Go tests
   (table-driven when it fits).
4. UI text goes through Transloco: add the key to **both**
   `src/i18n/en.json` and `src/i18n/fr.json` (French runs 15 to 20 % longer:
   check that layouts hold).
5. Use the glossary terms everywhere (types, functions, events, i18n keys,
   labels).
6. Significant architecture decisions get an ADR in `docs/adr`.
7. Run the checks above, then open a pull request against `main` and fill in
   the template.

### Commit messages

[Conventional Commits](https://www.conventionalcommits.org/):
`feat(engine): …`, `fix(desktop): …`, `docs: …`, `refactor: …`. Write them in
English, in the imperative mood, explaining why when it is not obvious.

### Sign your commits (DCO)

Every commit must be signed off, certifying that you wrote it or have the
right to submit it under the project license, as stated in the
[Developer Certificate of Origin](https://developercertificate.org/):

```bash
git commit -s -m "fix(desktop): keep the bib when a check-in is cancelled"
```

This adds a `Signed-off-by: Your Name <you@example.com>` line. A pull request
whose commits are not all signed off cannot be merged. Forgot it?
`git rebase --signoff main` then force-push your branch.

### AI-assisted contributions

They are welcome, under the same rules: you are responsible for every line you
submit, you have read and tested it, and you sign it off.

## Review

A maintainer reviews every pull request. CI must be green (Go tests, frontend
tests, i18n keys, formatting, sign-off). Pull requests are squashed on merge:
the title becomes the commit message, so make it a good Conventional Commit.

## License

OpenShuttle is licensed under the [GNU AGPL v3.0 or later](LICENSE). By
contributing, you agree that your contributions are licensed under the same
terms.
