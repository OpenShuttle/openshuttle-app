## What and why

<!-- What does this change do for the organizer or the control desk? Link the issue: "Closes #123". -->

## How it was tested

<!-- Tests added, and what you checked in the app (screens, both languages). -->

## Checklist

- [ ] Commits are signed off (`git commit -s`), see CONTRIBUTING.md
- [ ] The PR title is a Conventional Commit (`feat(engine): …`)
- [ ] Go changes come with tests; `cd src && go test ./...` passes
- [ ] UI text is in both `en.json` and `fr.json` (`npm run i18n:check`)
- [ ] Bindings regenerated if a Go service changed
- [ ] Event JSON fields unchanged, or the file migration is handled
- [ ] Docs updated (scope, ADR, CLAUDE.md) if behaviour or architecture changed
