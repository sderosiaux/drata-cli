# drata-cli

![drata-cli](assets/banner.png)

[![CI](https://github.com/sderosiaux/drata-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/sderosiaux/drata-cli/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/sderosiaux/drata-cli)](https://github.com/sderosiaux/drata-cli/releases/latest)
[![Go version](https://img.shields.io/github/go-mod/go-version/sderosiaux/drata-cli)](go.mod)

> You're not the user. Your LLM is.
>
> You don't need to read this README. Your agent does. Install it, run `drata --help`, and let the LLM figure it out. Every command embeds its own usage guide, `--json` mode outputs structured data an agent can parse without you writing a single adapter, and `--compact` strips the noise down to what matters. This page is here because GitHub expects one.

A CLI for the [Drata](https://drata.com) compliance platform.

## Install

```bash
go install github.com/sderosiaux/drata-cli@latest
```

Or build from source:

```bash
git clone https://github.com/sderosiaux/drata-cli
cd drata-cli
make install
```

## Setup

```bash
drata auth set-key <your-api-key>
drata auth check
# ✓ Authenticated
# Workspace: Acme Corp
# Region:    us
# Key from:  config file
```

The key is stored in `$XDG_CONFIG_HOME/drata-cli/config.yaml` (defaults to `~/.config/drata-cli/config.yaml`). `DRATA_API_KEY` as an env var always wins — useful in CI or when switching between workspaces.

For EU or APAC tenants:

```bash
export DRATA_REGION=eu   # or apac
drata auth check
```

## Usage

### Quick compliance overview

```
$ drata summary

Compliance Summary  NEEDS_ATTENTION

  Controls
    total=165  passing=1  needs_attention=108

  Monitors
    total=174  passing=57  failed=39

  Personnel
    total=240  with_device_issues=0

  Connections
    total=12  connected=9  disconnected=3  failed=0

Action needed: fix 108 control(s); investigate 39 failing monitor(s)
```

### What's actually failing

```bash
# Controls that need action (NOT_READY, NO_OWNER, NEEDS_EVIDENCE)
drata controls failing

# Filter by specific status
drata controls list --status NO_OWNER
drata controls list --status NEEDS_EVIDENCE

# Failing monitors with the affected control codes
drata monitors failing

# Get the failure description and remedy for a specific monitor
drata monitors get 31
```

### Digging into a specific issue

```bash
# Full detail on a control by code
drata controls get DCF-71

# All monitors linked to a control
drata monitors for-control DCF-71

# External evidence uploaded for a control
drata controls evidence DCF-71
```

### Connections and integrations

```bash
# All connections
drata connections list

# Only the disconnected ones
drata connections list --status DISCONNECTED

# CONNECTED, DISCONNECTED, FAILED
drata connections list --status FAILED
```

### Personnel

```bash
drata personnel list
drata personnel list --status CURRENT_EMPLOYEE
drata personnel get --email alice@example.com
drata personnel issues   # only those with device compliance failures
```

### Other resources

```bash
drata policies list
drata vendors list
drata devices list
drata devices list --user alice@example.com
drata assets list
drata assets list --type CLOUD
drata users list
drata events list
drata events list --category MONITOR
drata evidence list
drata evidence expiring --days 60
```

## LLM and script usage

Add `--json` to any command to get structured output. Add `--compact` on top of that to strip non-essential fields:

```bash
# Compact dashboard for a system prompt or tool call
drata summary --json --compact

# All failing controls with status breakdown
drata controls failing --json

# Specific monitor with failure description and remedy
drata monitors get 31 --json
```

Example output from `drata summary --json --compact`:

```json
{
  "status": "NEEDS_ATTENTION",
  "controls":    { "total": 165, "passing": 1,  "needs_attention": 108 },
  "monitors":    { "total": 174, "passing": 57, "failed": 39 },
  "personnel":   { "total": 240, "with_issues": 0 },
  "connections": { "total": 12,  "connected": 9, "disconnected": 3, "failed": 0 }
}
```

Use `--limit N` to cap results when you only need a sample:

```bash
drata controls list --status NEEDS_EVIDENCE --json --limit 10
```

Disable color for clean terminal output (also respected via `NO_COLOR` env):

```bash
drata monitors failing --no-color
```

## Global flags

| Flag | Description |
|------|-------------|
| `--json` | Output as JSON |
| `--compact` | Minimal fields only (requires `--json`) |
| `--limit N` | Cap the number of results |
| `--no-color` | Disable ANSI color output |
| `--region` | API region: `us`, `eu`, `apac` (default: `us`) |

## Status values

**Controls** — derived by the CLI from API fields:

| Status | Meaning |
|--------|---------|
| `PASSING` | Monitored and has evidence |
| `NEEDS_EVIDENCE` | Missing evidence upload |
| `NOT_READY` | Control not configured |
| `NO_OWNER` | No owner assigned |
| `READY` | Configured but not yet monitored |
| `ARCHIVED` | Excluded from compliance score |

**Monitors**: `PASSED`, `FAILED`, `NOT_TESTED`

**Connections**: `CONNECTED`, `DISCONNECTED`, `FAILED`

## Configuration

| Source | Format | Priority |
|--------|--------|----------|
| `DRATA_API_KEY` env var | string | highest |
| `DRATA_REGION` env var | `us`/`eu`/`apac` | high |
| `$XDG_CONFIG_HOME/drata-cli/config.yaml` | YAML | fallback |

Config file written by `drata auth set-key`:

```yaml
api_key: your-key-here
region: us   # optional
```

## API regions

| Region | Base URL |
|--------|----------|
| `us` (default) | `https://public-api.drata.com` |
| `eu` | `https://public-api.eu.drata.com` |
| `apac` | `https://public-api.apac.drata.com` |
