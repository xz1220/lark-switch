# AGENTS.md

Notes for AI agents working **in this repository**. For how to *call* lark-switch
as a tool (account selection, `run` vs `use`, JSON output), the contract lives in
[`SKILL.md`](SKILL.md) and [`references/`](references/) — read those, not this file.

## Repo layout

This repo is both a Go CLI (distributable binary) and a Claude skill/plugin.

```
SKILL.md               # the skill — usage contract for agents (canonical)
references/            # skill deep-dives, loaded on demand
  agent-playbook.md    #   multi-account operating procedure
  commands.md          #   full command + flag reference
  json-output.md       #   ls --json / current --json schemas
.claude-plugin/        # plugin.json + marketplace.json (installable from GitHub)
README.md              # human-facing docs
AGENTS.md              # this file — dev notes
main.go                # commands / CLI dispatch
store.go               # account registry + env construction (envForAccount)
status.go              # `lark-cli auth status` parsing
lark_test.go           # tests
```

## Build & test

- Go **stdlib only** — do not add dependencies.
- Build: `go build -o lark-switch .`  ·  Test: `go test ./...`  ·  Vet: `go vet ./...`
- Install: `cp lark-switch ~/.local/bin/` (or anywhere on PATH). The binary is
  gitignored; never commit it.
- Bump `version` in `main.go` and `version` in `.claude-plugin/plugin.json`
  together on a release.

## Invariants — don't break these

- **`use` emits shell code on stdout for `eval`; human messages go to stderr.**
  Anything `use` prints to stdout **is executed by the user's shell** — keep it
  valid shell and quote paths with `shellQuote`.
- **`use` must never claim success unless its output is really eval'd.** The shim
  (`shellenv`) sets `LARK_SWITCH_EVAL=1`; without it, `use` prints the working
  alternative and exits 1. This is what makes the tool safe for agents.
- **The default-home account never gets `LARKSUITE_CLI_CONFIG_DIR` set** — its
  tokens live under `~/Library/Application Support/lark-cli`. `envForAccount`
  (store.go) and `isDefaultHome` enforce this; keep them the single source.
- **`envForAccount` always strips any inherited `LARKSUITE_CLI_CONFIG_DIR` first**
  so a parent shell pinned to one account can't leak into a sub-invocation.
- Machine-readable surfaces (`ls --json`, `current --json`) are a public
  contract — additive changes only; update `references/json-output.md` alongside.

## Keep in sync when changing behavior

`SKILL.md`, `references/*.md`, `README.md`, and the `usage()` text in `main.go`
all describe the same commands. Change one, scan the others.
