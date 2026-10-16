# OpenShuttle

Open source desktop application to run non-sanctioned badminton club
tournaments, fully offline: check-in, draw, pools and brackets, court
management, match calls on a second screen, live corrections with undo.

Website: [openshuttle.app](https://openshuttle.app)

> Early development. Nothing is usable yet.

## Development

See [CLAUDE.md](CLAUDE.md) for the architecture and the commands, and
[docs/specs/cadrage-v1.md](docs/specs/cadrage-v1.md) for the functional spec
(in French).

```
cd src/apps/desktop
wails3 dev
```

## Versions

Versions are `MAJOR.MINOR.PATCH`. Apps sharing the same MAJOR work together
(tournament files, relay protocol); MINOR is each app's own version; PATCH is
for security fixes and dependency updates. Releases are published on GitHub
from tags such as `desktop-v1.0.0`: see [docs/releasing.md](docs/releasing.md).
