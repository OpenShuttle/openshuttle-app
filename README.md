# OpenShuttle

[![CI](https://github.com/OpenShuttle/openshuttle-app/actions/workflows/ci.yml/badge.svg)](https://github.com/OpenShuttle/openshuttle-app/actions/workflows/ci.yml)
[![Quality Gate](https://sonarcloud.io/api/project_badges/measure?project=OpenShuttle_openshuttle-app&metric=alert_status)](https://sonarcloud.io/summary/new_code?id=OpenShuttle_openshuttle-app)
[![Coverage](https://sonarcloud.io/api/project_badges/measure?project=OpenShuttle_openshuttle-app&metric=coverage)](https://sonarcloud.io/summary/new_code?id=OpenShuttle_openshuttle-app)
[![License: AGPL v3](https://img.shields.io/badge/license-AGPL--3.0-blue.svg)](LICENSE)

Open source desktop application to run non-sanctioned badminton club
tournaments, fully offline, on one PC and a second screen for the hall.

Website: [openshuttle.app](https://openshuttle.app)

> Early development: not ready for a real tournament yet.

## What it does

- **Prepare**: one or two categories (for example men's and women's doubles),
  entrants imported from a spreadsheet or typed, a club list, formats built
  from stages (pools, brackets, consolation) with presets and a simulator.
- **On the day**: check-in with bibs, draw with ranked seeds and club
  separation, a schedule that forecasts every match and drives the courts,
  score entry from the court, timed rounds with a golden point.
- **Display screen**: match calls and standings in large type on a second
  screen or projector.
- **Never stuck**: every action is recorded and can be undone; the tool warns
  but lets the desk decide. Corrections (scores, forfeits, swaps) at any time.
- **Results**: full standings, CSV export, archived tournament file.
- English and French, no network, no account, no printing.

The functional scope is in [docs/specs/scope-v1.md](docs/specs/scope-v1.md).

## Download

Releases for Windows, macOS and Linux are published on the
[Releases page](https://github.com/OpenShuttle/openshuttle-app/releases).

## Build from source

You need Go (version in [src/go.mod](src/go.mod)), Node 24 and the Wails CLI:

```bash
go install github.com/wailsapp/wails/v3/cmd/wails3@v3.0.0-beta.30
cd src/apps/desktop/frontend && npm ci
cd src/apps/desktop && wails3 dev      # or: wails3 build
```

On Linux, install `libgtk-4-dev libwebkitgtk-6.0-dev libsoup-3.0-dev` first.

## Contributing

Bug reports, ideas and pull requests are welcome: read
[CONTRIBUTING.md](CONTRIBUTING.md) first. Architecture rules are in
[CLAUDE.md](CLAUDE.md), decisions in [docs/adr](docs/adr). Everyone takes part
under the [Code of Conduct](CODE_OF_CONDUCT.md).

## Versions

Versions are `MAJOR.MINOR.PATCH`. Apps sharing the same MAJOR work together
(tournament files, relay protocol); MINOR is each app's own version; PATCH is
for security fixes and dependency updates. Releases are published from tags
such as `desktop-v1.0.0`: see [docs/releasing.md](docs/releasing.md).

## License

OpenShuttle is free software, licensed under the
[GNU Affero General Public License v3.0 or later](LICENSE).
