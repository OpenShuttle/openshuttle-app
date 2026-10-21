# Releasing

Versions are `MAJOR.MINOR.PATCH` (see [ADR 3](adr/0003-versioning-and-releases.md)):

- **MAJOR**: compatibility generation, shared by every app (`version.Compat` in `src/internal/version`).
- **MINOR**: the app's own version, for features and functional fixes.
- **PATCH**: security fixes and dependency updates only.

## Desktop app

1. Make sure `main` is green on CI.
2. Pick the version: previous `desktop-v1.3.0` + features → `1.4.0`; + only
   Renovate or security fixes → `1.3.1`.
3. Tag the commit of `main` and push the tag:

   ```bash
   git switch main && git pull
   git tag -a desktop-v1.4.0 -m "OpenShuttle desktop 1.4.0"
   git push origin desktop-v1.4.0
   ```

The [Release desktop](../.github/workflows/release-desktop.yml) workflow then:

- checks the tag (`desktop-vMAJOR.MINOR.PATCH`, MAJOR equal to `version.Compat`,
  commit on `main`);
- stamps the version with `src/apps/desktop/build/set-version.sh`;
- builds on Linux, Windows and macOS;
- publishes a GitHub release with notes generated from the merged PRs and:

  | File | Platform |
  | --- | --- |
  | `openshuttle-desktop-<v>-windows-amd64-setup.exe` | Windows installer (NSIS, installs WebView2 if missing) |
  | `openshuttle-desktop-<v>-windows-amd64-portable.zip` | Windows, no install |
  | `openshuttle-desktop-<v>-linux-amd64.deb` | Debian 13+ / Ubuntu 24.04+ (GTK 4, WebKitGTK 6) |
  | `openshuttle-desktop-<v>-linux-amd64.tar.gz` | Linux binary |
  | `openshuttle-desktop-<v>-macos-arm64.zip` | macOS app (Apple Silicon), ad-hoc signed |
  | `SHA256SUMS.txt` | Checksums |

### Release notes

The release description is generated from the commits since the previous
`desktop-v…` tag by [git-cliff](https://git-cliff.org) (`cliff.toml`): no
changelog file to maintain. Commits are grouped by their Conventional Commits
type (Features, Bug fixes, Performance, Refactoring, Documentation,
Dependencies; `chore`, `ci`, `build`, `test` and `style` are left out), and
breaking changes (`feat!:` or a `BREAKING CHANGE:` footer) come first. So the
commit subject is what users read: write it for them. Preview the next notes
with `npx git-cliff@2.14.2 --unreleased`.

A wrong tag fails the workflow before building: delete it (`git push origin
:refs/tags/desktop-vX.Y.Z`), fix, tag again.

### Download links

The website links to `openshuttle.app/download/<platform>` (or
`/telecharger/<platform>`), never to a release file: the last step of the
workflow points these links to the new release, through a Cloudflare Bulk
Redirects list (302 redirects). Platforms: `windows`, `windows-portable`,
`macos`, `linux-deb`, `linux-tar`, `checksums`. Releases of the other apps
(relay, web) never touch them.

One-time setup in Cloudflare (the zone `openshuttle.app` on the account):

1. **Rules → Bulk Redirects**: create a list `openshuttle_downloads`, then a
   Bulk Redirect rule using it. Note the list ID (in the list's URL, or `GET
   /accounts/<account>/rules/lists`).
2. **My Profile → API Tokens**: create a token with only *Account → Account
   Filter Lists → Edit* on this account.
3. In the GitHub repository, **Settings → Secrets and variables → Actions**,
   add `CLOUDFLARE_API_TOKEN`, `CLOUDFLARE_ACCOUNT_ID` and
   `CLOUDFLARE_DOWNLOADS_LIST_ID`.

Until the three secrets exist, the step is skipped and the links stay as they
are. Each release replaces the whole list.

### Not signed yet

The Windows installer and the macOS app are not signed with a publisher
certificate: Windows SmartScreen and macOS Gatekeeper warn on first launch.
On Windows: More info → Run anyway. On macOS (ad-hoc signed), since macOS 15:
open the app once, then System Settings → Privacy & Security → Open Anyway.
Planned: Windows signing through SignPath Foundation (free for open source,
needs a code signing policy page on the website); macOS needs an Apple
Developer account (or a fee waiver for a non-profit).

## Changing MAJOR

Only when tournament files or the relay protocol break compatibility: bump
`Compat` in `src/internal/version/version.go`, provide the file migration,
then release every app with the new MAJOR.

## Dependencies

Renovate (`renovate.json`) opens grouped PRs on Monday mornings, 7 days after
a release (security fixes at once). It needs the
[Renovate GitHub app](https://github.com/apps/renovate) installed on the
repository; the first run opens an onboarding PR and a Dependency Dashboard
issue.
