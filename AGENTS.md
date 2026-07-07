# AGENTS.md

Notes for AI agents working **in this repository**. For how to *call* lark-switch
as a tool (account selection, `run` vs `use`, JSON output), the contract lives in
[`SKILL.md`](SKILL.md) and [`references/`](references/) — read those, not this file.

## Repo layout

This repo is both a Go CLI (distributable binary) and a Claude skill/plugin.
The root is kept skill-facing; all Go lives under `src/`.

```
SKILL.md               # the skill — usage contract for agents (canonical)
references/            # skill deep-dives, loaded on demand
  agent-playbook.md    #   multi-account operating procedure
  commands.md          #   full command + flag reference
  json-output.md       #   ls --json / current --json schemas
.claude-plugin/        # plugin.json + marketplace.json (installable from GitHub)
.github/workflows/     # ci.yml (build+test), release.yml (cross-compile + publish)
install.sh             # one-line installer: fetches a prebuilt release binary
README.md              # human-facing docs
AGENTS.md              # this file — dev notes
src/                   # the Go module (self-contained)
  go.mod
  main.go              #   commands / CLI dispatch
  store.go             #   account registry + env construction (envForAccount)
  status.go            #   `lark-cli auth status` parsing
  lark_test.go         #   tests
```

## Build & test

Run Go commands from `src/` (the module root):

- Go **stdlib only** — do not add dependencies.
- Build: `cd src && go build -o lark-switch .`
- Test: `cd src && go test ./...`  ·  Vet: `cd src && go vet ./...`
- Install locally: `cp src/lark-switch ~/.local/bin/`. The binary is gitignored;
  never commit it.

## Releasing

- `version` in `main.go` is `"dev"` by default and injected at build time via
  `-ldflags "-X main.version=<tag>"` — don't hardcode it per release.
- Tag `vX.Y.Z` and push the tag: `release.yml` cross-compiles darwin/linux ×
  amd64/arm64, packages `lark-switch_<tag>_<os>_<arch>.tar.gz` + `checksums.txt`,
  and publishes a GitHub Release. `install.sh` resolves the latest tag and
  downloads the matching asset — keep the asset naming and the script in sync.
- Bump `version` in `.claude-plugin/plugin.json` on a release (skill version).

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
