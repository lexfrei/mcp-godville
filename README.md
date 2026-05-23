# MCP Godville

[![Go Version](https://img.shields.io/github/go-mod/go-version/lexfrei/mcp-godville)](https://go.dev/)
[![License](https://img.shields.io/github/license/lexfrei/mcp-godville)](LICENSE)
[![Release](https://img.shields.io/github/v/release/lexfrei/mcp-godville)](https://github.com/lexfrei/mcp-godville/releases)
[![Build](https://github.com/lexfrei/mcp-godville/actions/workflows/release.yaml/badge.svg)](https://github.com/lexfrei/mcp-godville/actions/workflows/release.yaml)

MCP server for the [Godville](https://godville.net) zero-player game API. Lets LLMs inspect a hero's status, diary, inventory, pet, quest, long-term progress and clan via the Model Context Protocol.

## Features

- **Hero state** — status (level, alignment, health, godpower), pet, inventory, quest, clan, long-term progress.
- **Diary access** — last diary entry and "third eye" log line (requires userkey).
- **Raw payload** — escape hatch tool exposing the full Godville JSON for anything not yet modelled.
- **In-memory cache** — Godville data updates once per minute upstream and enforces a 30 req / 10 min rate limit per (god+ip). With the default 60s TTL the worst case is 1 fetch per minute per (godname, userkey) = 10 fetches per 10-minute window per credential = 1/3 of the budget for a single hero. Concurrent tool calls coalesce via singleflight so a 9-tool LLM burst is one upstream fetch, not nine.
- **Two-mode auth** — credentials from env or MCP elicitation. Without a userkey, public-only fields are exposed; with one, private fields appear too.
- **Multi-arch images** — `linux/amd64` and `linux/arm64`, signed with cosign keyless.
- **Both Godville variants** — Russian (default) and English via `GODVILLE_API_BASE`.

## Quick Start

By default the server resolves the godname and (optional) userkey via interactive **MCP elicitation** on the first tool call — same flow as `mcp-tg`. The MCP client prompts the user in-place; nothing needs to be pre-configured. Env vars are a non-interactive alternative for headless / CI deployments.

### Container (Podman/Docker)

```json
{
  "mcpServers": {
    "godville": {
      "command": "podman",
      "args": [
        "run", "--rm", "-i",
        "ghcr.io/lexfrei/mcp-godville:0.1.0"
      ]
    }
  }
}
```

> Claude Code's project-level `.mcp.json` uses a flat top-level (`{"mcp-godville": {...}}`) instead of the `mcpServers` wrapper shown above — see the repo's `.mcp.json` for that shape. Other MCP clients (Cursor, Claude Desktop, etc.) use the `mcpServers` form. Pick whichever matches your client.

The first tool call triggers elicitation: the client asks for the god name, then (optionally) the userkey. To skip the prompts, pass `-e GODVILLE_GODNAME=YourGod -e GODVILLE_USERKEY=your-key` in `args` — the server uses env values when set and only elicits the missing pieces.

> Pin a numeric version tag (not `:latest`) — `:latest` can be retagged on top of any image after the fact, defeating the cosign supply-chain assertion shown in the Verification section. The `latest` tag is published for convenience only; production deployments should pin.

### Go Install

```bash
go install github.com/lexfrei/mcp-godville/cmd/mcp-godville@latest
```

```json
{
  "mcpServers": {
    "godville": {
      "command": "mcp-godville"
    }
  }
}
```

Same elicitation flow as the container example. Override with `env.GODVILLE_GODNAME` / `env.GODVILLE_USERKEY` to skip the interactive prompts.

## Configuration

All configuration is via environment variables. Both credentials may also be supplied interactively through MCP elicitation if not pre-configured.

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| `GODVILLE_GODNAME` | Yes (env or elicit) | — | Hero's god name (URL path segment) |
| `GODVILLE_USERKEY` | No | — | Userkey for private API fields (diary, quest, health) |
| `GODVILLE_API_BASE` | No | `https://godville.net` | API base URL. Use `https://godvillegame.com` for the English variant |
| `GODVILLE_CACHE_TTL` | No | `60s` | In-memory cache TTL (Go duration). Anything ≤60s wastes calls — upstream only refreshes once a minute |
| `MCP_HTTP_PORT` | No | — | Enable HTTP transport on this port (in addition to stdio). The HTTP transport has **no built-in authentication** — bind scope is the only access control. When running in a container, publish with `-p 127.0.0.1:PORT:PORT` (not `-p PORT:PORT`, which would bind to `0.0.0.0` on the host network) |
| `MCP_HTTP_HOST` | No | `127.0.0.1` | HTTP bind address |

> **HTTP transport is single-tenant.** It shares the credentials elicited via the stdio peer (or set via env) — all HTTP callers see the same hero. There is no per-caller credential elicitation. When enabling HTTP, prefer setting `GODVILLE_GODNAME` (and optionally `GODVILLE_USERKEY`) via env so the credentials are resolved before any HTTP request arrives.

### Public vs Private mode

The Godville API exposes two tiers (Russian Godville only — `godvillegame.com` does not have a private API):

- **Public** (no userkey): identity, level, alignment, motto, clan, pet (name/class/level), arena, ark, bricks, wood, savings, t-level, shop name, `inventory_max_num` (the cap, NOT the live fill count), milestone completion timestamps, boss name, **boss_power**.
- **Private** (with userkey): everything above + `health`, `godpower`, `diary_last`, `eye_last`, `quest`, `quest_progress`, `side_job`, `side_job_progress`, `distance`, `town_name`, `fight_type`, `arena_fight`, `inventory_num`, `activatables`, `aura`, `gold_approx`, `exp_progress`. The legacy itemised `inventory` field is deprecated upstream and usually empty on current accounts — use `activatables` for the trophy list.

A userkey can be obtained from the hero's settings page on godville.net.

## Available Tools

All tools are read-only and require no parameters. The hero is configured server-side.

### `hero_status`

Current vital stats: name, godname, level, alignment, motto, health/maxHealth, godpower, fight type, location, distance.

### `hero_diary`

Last diary entry and "third eye" log line. Empty (with a hint) in public mode.

### `hero_inventory`

Inventory usage (`count`, `max`, `distinct`), the legacy itemised `items` map (kept for backward compat — deprecated upstream and usually empty on current accounts), and the current `activatables` list (trophies that can be used in combat). All non-counter fields require userkey.

### `hero_pet`

Pet's name, class, level, wounded state.

### `hero_quest`

Battle and quest context. **Boss name and boss power are public**; active quest + progress, side job + progress, current town, distance, fight type, and arena-fight flag require a userkey.

### `hero_progress`

Long-term progress trackers: gold, savings, bricks, wood, arena W/L, t-level, ark headcount, souls/relics percent, words, milestone completion timestamps, shop name.

### `hero_clan`

Clan membership and in-clan rank.

### `hero_raw`

Full raw Godville API JSON for the configured hero. Use to inspect fields not exposed by the typed tools.

### `server_version`

Server build info: version, git revision, Go runtime. Does not call the Godville API.

## Architecture

```text
cmd/mcp-godville/main.go         Entry point: config → client → cache → service → tools
internal/config/                  Env var loading and validation
internal/godville/                HTTP client + types + in-memory cache
  client.go                       GetHero(godname, userkey) → *Hero
  types.go                        Hero/Pet/ErrorPayload + raw payload preservation
  cache.go                        Per-(godname,userkey) TTL cache + singleflight
internal/auth/                    Credential resolution: env → MCP elicitation → error/public
internal/heroservice/             Glue between auth and cache; implements tools.HeroProvider
internal/tools/                   MCP tool handlers (one file per tool + test)
```

The tools depend on a single `HeroProvider` interface, which `heroservice.Service` implements by combining the authenticator and the cache. `main.go` wires the parts together via `heroservice.New(authenticator, cache)`. Tests stub the interface directly — no HTTP server needed for tool-level tests.

## Development

```bash
go build ./...
go test -race ./...
golangci-lint run
```

TDD throughout: tests live next to implementation in the same package directory and were written first.

## Verification

Container images are signed with cosign keyless signing:

```bash
# Pin to a release tag so the cosign assertion ties to a specific build.
# Verifying ":latest" works but defeats the supply-chain guarantee —
# ":latest" can be re-tagged on top of any image after the fact.
# NOTE: docker/metadata-action's semver pattern strips the "v" prefix,
# so git tag v0.1.0 → image tag 0.1.0 (no "v").
cosign verify ghcr.io/lexfrei/mcp-godville:0.1.0 \
  --certificate-identity-regexp='^https://github\.com/lexfrei/mcp-godville/' \
  --certificate-oidc-issuer=https://token.actions.githubusercontent.com
```

## License

[BSD-3-Clause](LICENSE)
