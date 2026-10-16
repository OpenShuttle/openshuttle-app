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

A wrong tag fails the workflow before building: delete it (`git push origin
:refs/tags/desktop-vX.Y.Z`), fix, tag again.

### Not signed yet

The Windows installer and the macOS app are not signed with a publisher
certificate: Windows SmartScreen and macOS Gatekeeper warn on first launch
(macOS: right click → Open). Signing needs certificates and secrets, to set up
before a public release.

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
