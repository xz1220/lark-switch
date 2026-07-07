---
name: lark-switch
description: 用 lark-switch 选定某个飞书/Lark 账号（多租户）来执行 lark-cli 命令。当任务涉及多个飞书账号、需要以某个指定账号执行 lark-cli 或 lark-* skill、用户提到 lark-switch / 切换账号 / 多账号 / 别的租户身份，或需要确认当前命令实际跑在哪个账号上时使用。不负责 lark-cli 本身的登录授权排障（走 lark-shared）。
license: MIT
metadata:
  trigger: 需要以某个指定飞书账号执行 lark-cli / lark-* 操作，或要确认命令跑在哪个账号上时。
  hard_rule: 在 Agent 工具调用里选账号只用 `lark-switch run <name> -- ...`；绝不用 `use` 或 `lark-cli profile use`。
  source: 从 lark-switch README 的「Agent 操作多账号的实践说明」提炼并固化。
---

# lark-switch

`lark-cli` 一次只有一个活跃身份，`lark-cli profile use` 又是**全局**切换（改写共享的 `~/.lark-cli/config.json`），并行的 shell / Agent 会互相踩。`lark-switch` 把每个账号钉在各自的配置目录（`LARKSUITE_CLI_CONFIG_DIR`）上，让多个账号并行共存、互不干扰。二进制在 `PATH` 上（源码见本仓库）。

## 何时使用

任务里出现「用某个账号 / 某个租户 / 某个身份去做飞书操作」，或要确认「这条命令现在是谁在跑」时，就用本 skill。**每条关键 `lark-cli` 命令都要显式绑定账号**——不要假设账号选择会在多次工具调用之间保留。

## 铁律（Agent 必读）

1. **选账号只用 `run`：`lark-switch run <name> -- <lark-cli 参数>`。** 它是无状态的，一次只作用于这一条命令，并行安全。
2. **绝不用 `lark-switch use`，也绝不用 `lark-cli profile use`。** `use` 需要交互 shell 的 shim，在工具调用里只会打印环境变量并**直接报错退出（exit 1）**，不会真的切换；`profile use` 改写全局状态，会波及并行会话。
3. **不要跨命令继承账号。** 每个工具调用都是新的子进程，环境不会带过来。要么 `run <name> --`，要么在同一条命令里带 `LARKSUITE_CLI_CONFIG_DIR=...` 前缀。
4. **默认账号很特殊。** 目录为 `~/.lark-cli`（`ls --json` 里 `"default_home": true`）的账号**不能**手动把 `LARKSUITE_CLI_CONFIG_DIR` 指向它，否则会显示未登录。走 `run <name> --` 即可，`run` 会自动处理好。

## 标准流程

**1. 查清有哪些账号、谁是默认**（Agent 用 `--json`，含 `default_home`、token 状态、剩余续期时间）：

```sh
lark-switch ls --json
```

**2. 动作前先验证身份**（改权限、发消息等敏感操作前必做）：

```sh
lark-switch run <name> -- whoami
```

**3. 每条命令绑定账号执行**：

```sh
lark-switch run <name> -- im +chat-list --as user --types group --sort active_time
lark-switch run <name> -- drive +member-add --token "https://xxx.feishu.cn/wiki/..." \
  --member-id "ou_a,ou_b" --member-type openid --perm full_access --perm-type container --yes
```

需要连续跑多条时，也可以显式带账号目录（`path` 打印绝对路径）：

```sh
LARKSUITE_CLI_CONFIG_DIR="$(lark-switch path <name>)" lark-cli whoami
```

## 反例（不要这样写）

```sh
# ❌ use 在工具调用里不会切换，且 exit 1；下一条命令仍是原账号
lark-switch use dangdang
lark-cli whoami
```

正确写法是把两步合成一条：`lark-switch run dangdang -- whoami`。

## 和 lark-* skill 配合

所有 `lark-*` skill 底层都调 `lark-cli`，默认跑在当前默认账号。要以其他账号执行时，把该 skill 给出的 `lark-cli <args>` 改写成 `lark-switch run <name> -- <args>`（去掉 `lark-cli` 前缀），或加 `LARKSUITE_CLI_CONFIG_DIR` 前缀。

整个 Agent 会话也可以在启动时绑定到一个账号：用 `LARKSUITE_CLI_CONFIG_DIR=$(lark-switch path <name>)` 启动，会话内每次 `lark-cli` 调用都固定到该账号（默认账号无需设变量）。

## 常用命令

| 命令 | 作用 |
|---|---|
| `lark-switch ls [--json]` | 列出账号、身份、token 状态、剩余续期（`*` 为当前） |
| `lark-switch run <name> -- <args>` | 以指定账号执行一条 lark-cli 命令（无状态，Agent 首选） |
| `lark-switch current [--json]` | 当前进程环境对应的账号 |
| `lark-switch path <name>` | 打印账号的配置目录绝对路径 |
| `lark-switch each -- <args>` | 对所有账号各执行一遍 |
| `lark-switch refresh --all` | 续活所有账号 token（token 闲置 ~7 天会失效） |
| `lark-switch add <name> [--dir <path>] [--init]` | 注册新账号 |

## references

- `references/agent-playbook.md` — Agent 多账号操作的完整实践：先查后验、逐条绑定、默认账号处理、会话级绑定、常见坑。
- `references/commands.md` — 全部命令与 flag 的详细说明。
- `references/json-output.md` — `ls --json` / `current --json` 的字段结构，供程序化解析。
- 开发本仓库见根目录 `AGENTS.md`。
