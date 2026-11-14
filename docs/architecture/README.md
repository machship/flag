---
lastReviewed: 2026-11-14
---
# flag

`github.com/machship/flag` is MachShip's fork of [namsral/flag](https://github.com/namsral/flag),
a drop-in replacement for Go's standard `flag` package that also reads environment variables
and a config file. It is not deployed; services import it. Owned by uplift.

## What MachShip changed

Upstream stopped in 2017 (`v1.7.4-pre`). MachShip changed the module path to
`github.com/machship/flag` and added, in Sep 2025:

- **Layered sources**, highest wins: command line, environment (with optional prefix),
  secret directory (`-secret-dir`, one file per flag), config file (`-config`), defaults.
- **`@file` indirection** for any value, so a flag can point at a mounted secret file.
- **`ParseStruct`**: registers flags from struct tags (`flag`, `default`, `help`, `required`,
  `enum`, `sep`, `layout`, `min`, `max`, `pattern`, `deprecated`) and validates them.
- **`sensitive` tag**: masks a value in usage text, errors and introspection.
- **Extra types**: time, duration, decimal (shopspring), UUID, IP/CIDR, URL, byte size,
  regex, raw JSON, enums, slices and maps.
- **Registry / introspection** and **hot reload** of the config file through fsnotify.
- Fuzz and coverage tests.

Upstream is kept as the `upstream` git remote; nothing has been merged back.

## Who imports it

Checked `go.mod` in every sibling clone and GitHub code search (Nov 2026):

| Module | Repos |
|---|---|
| `github.com/machship/flag` (this fork) | aiagent (archived) at `v0.250905.1` |
| `github.com/namsral/flag v1.7.4-pre` (upstream, not this fork) | v2-go-cache (machship-v2-go-cache), v2-go-cache-invalidation, client-data-loader, modbot, tplus-pricer, capital-qbr-go, internal-zoho, tahoe-remotego, v3-local-nginx-docker, v3-backend-services/inquisitor |

No repo has a `replace github.com/namsral/flag => github.com/machship/flag` directive, so the
upstream users get none of the changes above. Switching is a path change plus checking that
`ParseEnv`/`ParseFile` behave the same.

## Context

See [diagrams/context.md](diagrams/context.md). No live component on the map imports the fork.

## Publishing

No CI and no release workflow. A maintainer pushes a date-style tag (`v0.YYMMDD.N`);
consumers fetch it with `GOPRIVATE=github.com/machship`.

## Running in k8s

Nothing. No image, no workload, no secrets.

## Lifecycle notes

Marked `ga` from the GitHub project-type, but its only importer is archived.
TODO(confirm): adopt it in the services still on upstream `namsral/flag`, or mark it `deprecated`.
