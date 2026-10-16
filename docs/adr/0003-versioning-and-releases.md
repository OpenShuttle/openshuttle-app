# 3. Versioning and releases

- Status: accepted
- Date: 2026-10-16

## Context

OpenShuttle will ship several apps from this repository: the desktop app now,
then the relay server and the website (v2). A club must be able to tell at a
glance which versions work together: which desktop can open a tournament file,
which relay a desktop can publish to.

## Decision

Every app is versioned `MAJOR.MINOR.PATCH`, with a meaning of its own:

| Part | Meaning | Changes when |
| --- | --- | --- |
| MAJOR | Compatibility generation, **the same for every app** | Tournament files or the relay protocol stop being compatible. Every app moves to the new MAJOR together. |
| MINOR | Version of the app itself | The app gets features or functional fixes. Each app counts its own. |
| PATCH | Maintenance | Security fixes and dependency updates only (Renovate PRs), no functional change. |

Apps with the same MAJOR work together, whatever their MINOR and PATCH:
desktop `1.4.2` and relay `1.0.7` are compatible; desktop `2.0.0` is not
compatible with relay `1.x`.

- The MAJOR lives in one place: `version.Compat` in `src/internal/version`.
  Release tooling refuses a version whose MAJOR differs.
- Each app is released by its own tag on a commit of `main`:
  `desktop-v1.0.1`, later `relay-v1.0.0` and `web-v1.0.0`. The prefix tells
  which app; the version follows the rules above.
- A release build stamps its version into the app (`version.txt` for the
  desktop) and the packaging metadata. Builds outside a release report `dev`.
- Dependencies are kept up to date by Renovate, grouped by ecosystem (Wails
  Go module, npm runtime and CLI move together), held 7 days after release
  except for vulnerability fixes.

## Consequences

- Bumping `version.Compat` is a deliberate, tournament-wide event: it needs a
  migration story for files and a coordinated release of every app.
- A merged Renovate PR does not release anything by itself: the next tag
  bumps PATCH.
- Tags are the source of truth for versions; nothing in `main` is edited by
  hand to release.
