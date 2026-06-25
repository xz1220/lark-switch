# lark-switch

A tiny **gvm-style multi-account switcher** for [`lark-cli`](https://github.com/larksuite/cli) (the Lark/Feishu CLI).

`lark-cli` only keeps one *active* identity at a time, and `lark-cli profile use`
switches it **globally** (it rewrites the shared `~/.lark-cli/config.json`), which
races across parallel shells and agent sessions. `lark-switch` instead pins each
account to **its own config home** via the `LARKSUITE_CLI_CONFIG_DIR` environment
variable, so two Feishu accounts — typically in two different tenants, each with
its own self-built app — coexist and can be used **in parallel** with no shared
mutable state.

```
$ lark-switch ls
   NAME       USER           STATUS         REFRESH-IN  DIR
*  A          邢政           valid          6d22h       ~/.lark-cli
   B          公司B           valid          6d20h       ~/.lark-cli-B
```

## How it works

Each account = a `name → config-home dir` mapping (stored in
`~/.config/lark-switch/config.json`). To act as an account, `lark-switch` runs
`lark-cli` with `LARKSUITE_CLI_CONFIG_DIR` set to that dir.

> **The default-home account is special.** The account whose dir is `~/.lark-cli`
> (lark-cli's built-in default) must **never** have `LARKSUITE_CLI_CONFIG_DIR` set:
> its encrypted tokens live under `~/Library/Application Support/lark-cli`, not under
> `~/.lark-cli`, so pinning the var would make it look logged-out. `lark-switch`
> detects this and leaves the var unset (and `use` emits `unset …`) for it. This
> lets you adopt your **existing** logged-in account as-is, with zero migration.

Two ways to select an account:

- **`run` (stateless, parallel-safe, agent-friendly)** — sets the env var for one
  command only: `lark-switch run B -- im +chat-list`. No global state changes.
- **`use` (per-shell switch, gvm-style)** — needs the shell shim; `lk use B`
  changes only the *current* shell. A compiled binary can't mutate its parent
  shell, so (like gvm/nvm/direnv) `use` prints `export …` that a shell function
  `eval`s — see [`shellenv`](#shell-integration).

## Install

```sh
cd ~/repos/lark-switch
go build -o lark-switch .
cp lark-switch ~/go/bin/      # or anywhere on your PATH
```

### Shell integration

Add to `~/.zshrc` (or `~/.bashrc`):

```sh
eval "$(lark-switch shellenv)"
```

This defines an `lk` function: `lk use B` switches the shell; everything else
(`lk ls`, `lk run A -- …`) passes straight through to `lark-switch`.

## Quickstart (two accounts, different tenants)

```sh
# 1. Adopt your existing logged-in account as the default home (no re-login):
lark-switch add A --dir ~/.lark-cli --note "me / tenant A"

# 2. Add the second account in its own home and log it in.
#    (First create a self-built app in account B's Feishu admin console and have
#     its App ID + App Secret ready; --init runs `config init` + `auth login`.)
lark-switch add B --init

# 3. Check both:
lark-switch ls
```

Daily use:

```sh
lk use B                       # this shell now targets account B
lark-cli calendar +agenda      # …runs as B

lark-switch run A -- mail +list  # one-off as A, without leaving B
lark-switch each -- auth status  # run a command across all accounts
```

## Parallel Claude Code / agent sessions

Bind a whole session to an account by launching it with the env var (the default
account needs no var):

```sh
claude                                          # session uses account A (default)
LARKSUITE_CLI_CONFIG_DIR=$(lark-switch path B) claude   # session uses account B
```

Every `lark-cli` call (and every `lark-*` skill) inside that session is then
pinned to the right account. Prefer this (or `run`) over `use` for agents —
`use`/`profile use` mutate shared state and can surprise a parallel session.

## Keep tokens alive

`lark-cli` user tokens use a **rolling ~7-day** refresh window that auto-extends on
any successful call; if an account goes untouched for >7 days you must re-login
(QR/device flow). Keep them warm with a cron job:

```cron
# refresh every account daily at 09:00
0 9 * * *  /Users/you/go/bin/lark-switch refresh --all >/dev/null 2>&1
```

## Commands

| command | what |
|---|---|
| `add <name> [--dir <path>] [--init] [--domain ...]` | register an account; `--init` runs `config init` + `auth login` |
| `login <name> [--domain ...\|--scope ...] [--init]` | (re)authorize an account |
| `ls` | list accounts with user, token status, refresh window (current marked `*`) |
| `use <name>` | switch the current shell (needs the shim) |
| `run <name> -- <args>` | run one `lark-cli` command as `<name>` |
| `each -- <args>` | run a command across all accounts |
| `refresh [<name>\|--all]` | keep tokens alive |
| `current` / `which` | show the account active in this shell |
| `path [<name>]` | print an account's config home dir |
| `rm <name> [--purge]` | unregister (`--purge` also deletes its home) |
| `shellenv` | print the shell shim |

## Notes

- Zero dependencies (Go stdlib only). macOS-focused (the default-home path uses
  `~/Library/Application Support`), but the mechanism works anywhere `lark-cli` runs.
- `NO_COLOR` is honored; output is plain when piped.
