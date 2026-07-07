# lark-switch — notes for AI agents

`lark-switch` pins each Feishu/Lark account to its own `lark-cli` config home so
multiple accounts can be used in parallel. This file covers how to *call* it from
an agent, and how to work on the repo itself.

## Using lark-switch in tool calls

Every tool call runs in a fresh child process. Environment changes do **not**
carry over to the next call, so account selection must be explicit **per command**.

```sh
# 1. Discover accounts (machine-readable):
lark-switch ls --json

# 2. Verify the identity you are about to act as (do this before privileged ops):
lark-switch run <name> -- whoami

# 3. Run each lark-cli command pinned to an account:
lark-switch run <name> -- im +chat-list --as user --types group
```

Rules:

- **Never use `lark-switch use` or `lark-cli profile use` in tool calls.** `use`
  requires the interactive shell shim; run anywhere else it changes nothing and
  exits 1. `profile use` rewrites global state shared with parallel sessions.
- **Never assume account selection persists across tool calls.** Pin every
  command with `run <name> --`, or inline the env var:
  `LARKSUITE_CLI_CONFIG_DIR="$(lark-switch path <name>)" lark-cli <args>`.
- **The default-home account (dir `~/.lark-cli`, `"default_home": true` in
  `ls --json`) must never have `LARKSUITE_CLI_CONFIG_DIR` pointed at it** — its
  tokens live elsewhere and it would look logged-out. Always go through
  `run <name> --` for it; `run` handles the var correctly for every account.
- A whole agent session can be bound to one account by launching it with
  `LARKSUITE_CLI_CONFIG_DIR=$(lark-switch path <name>)` in the environment; the
  default account needs no var.

Machine-readable surfaces: `ls --json`, `current --json`, `path <name>` (plain
absolute path). Errors go to stderr with exit code 1; `NO_COLOR` and non-TTY
output are plain text.

## Working on this repo

- Go stdlib only — do not add dependencies.
- Build: `go build -o lark-switch .` · Test: `go test ./...`
- `main.go` = commands/CLI, `store.go` = account registry (+ env construction),
  `status.go` = `lark-cli auth status` parsing.
- `use` emits shell code on stdout for `eval`; human messages go to stderr.
  Anything printed to stdout by `use` **is executed by the user's shell** — keep
  it valid shell, quote with `shellQuote`.
- The shim (`shellenv`) sets `LARK_SWITCH_EVAL=1` so the binary knows its stdout
  is really being eval'd; without it `use` fails loudly on purpose.
