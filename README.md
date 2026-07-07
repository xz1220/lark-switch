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
  `eval`s — see [`shellenv`](#shell-integration). Run *without* the shim (e.g.
  in a script or an agent tool call) `use` refuses to pretend: it prints what
  to do instead and exits 1.

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

### As a Claude skill / plugin

This repo ships a [`SKILL.md`](SKILL.md) (+ [`references/`](references/)) so AI
agents know how to drive `lark-switch` correctly — pick an account per command
with stateless `run`, read `ls --json`, never `use` inside a tool call. It
assumes the `lark-switch` **binary is already on PATH** (build + install above).

In Claude Code:

```bash
/plugin marketplace add xz1220/lark-switch
/plugin install lark-switch
```

Or clone into any agent's skills directory (`~/.claude/skills/lark-switch`,
`~/.codex/skills/lark-switch`, …):

```bash
git clone --depth 1 https://github.com/xz1220/lark-switch.git <skills-dir>/lark-switch
```

The agent-facing operating rules live in
[`references/agent-playbook.md`](references/agent-playbook.md).

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

Agent-facing surfaces: `ls --json` / `current --json` for machine-readable
state, and [AGENTS.md](AGENTS.md) for the full ruleset an agent should follow.

## Agent 操作多账号的实践说明

这次用 `lark-switch` 给同一个飞书文档处理不同账号、不同群聊的权限时，最稳定的流程是先把账号状态查清楚，再把每条 `lark-cli` 命令固定到某个账号上执行。

先用 `lark-switch ls` 看有哪些账号、哪个是当前默认账号（Agent 建议用 `--json`）：

```sh
lark-switch ls --json
```

然后用 `whoami` 验证当前命令实际跑在哪个账号上：

```sh
lark-cli whoami
lark-switch run dangdang -- whoami
```

对 Agent 来说，优先使用 `run` 做一次性调用：

```sh
lark-switch run dangdang -- im +chat-list --as user --types group --sort active_time
lark-switch run dangdang -- im chat.members get --chat-id oc_xxx --member-id-type open_id --page-all
lark-switch run dangdang -- drive +member-add --token "https://xxx.feishu.cn/wiki/..." \
  --member-id "ou_a,ou_b" --member-type openid --perm full_access --perm-type container --yes
```

如果需要连续执行多条命令，也可以显式传账号目录：

```sh
LARKSUITE_CLI_CONFIG_DIR="$(lark-switch path dangdang)" lark-cli whoami
LARKSUITE_CLI_CONFIG_DIR="$(lark-switch path dangdang)" lark-cli im +chat-list --as user --types group
```

需要注意：在 Codex、Claude Code 这类 Agent 的工具调用里，`lark-switch use dangdang` 不能改变后续工具调用的父 shell 环境（在 shim 之外运行时它会直接报错退出）。因此不要写成：

```sh
lark-switch use dangdang
lark-cli whoami
```

也不要假设多条工具调用会继承上一条的账号切换。每条关键命令都应该用 `lark-switch run <name> -- ...`，或者在同一条命令里带上 `LARKSUITE_CLI_CONFIG_DIR=...`。默认账号 `~/.lark-cli` 比较特殊，手动设置环境变量可能让它看起来像未登录；因此默认账号也建议通过 `lark-switch run A -- ...` 进入。

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
| `ls [--json]` | list accounts with user, token status, refresh window (current marked `*`) |
| `use <name>` | switch the current shell (needs the shim; exits 1 elsewhere) |
| `run <name> -- <args>` | run one `lark-cli` command as `<name>` |
| `each -- <args>` | run a command across all accounts |
| `refresh [<name>\|--all]` | keep tokens alive |
| `current [--json]` / `which` | show the account active in this shell |
| `path [<name>]` | print an account's config home dir |
| `rm <name> [--purge]` | unregister (`--purge` also deletes its home) |
| `shellenv` | print the shell shim |

## Notes

- Zero dependencies (Go stdlib only). macOS-focused (the default-home path uses
  `~/Library/Application Support`), but the mechanism works anywhere `lark-cli` runs.
- `NO_COLOR` is honored; output is plain when piped.
