# lark-switch

**中文** · [English](README.en.md)

一个 **gvm 风格的多账号切换器**，服务于 [`lark-cli`](https://github.com/larksuite/cli)（Lark/飞书 CLI）。

`lark-cli` 一次只有一个*活跃*身份，而 `lark-cli profile use` 是**全局**切换（改写共享的 `~/.lark-cli/config.json`），并行的 shell 和 Agent 会话会互相踩。`lark-switch` 把每个账号钉在**各自的配置目录**上（通过 `LARKSUITE_CLI_CONFIG_DIR` 环境变量），于是两个飞书账号——通常在两个不同租户、各有自建应用——可以**并行**共存、没有共享可变状态。

```
$ lark-switch ls
   NAME       USER           STATUS         REFRESH-IN  DIR
*  A          邢政           valid          6d22h       ~/.lark-cli
   B          公司B           valid          6d20h       ~/.lark-cli-B
```

## 工作原理

每个账号就是一条 `名字 → 配置目录` 的映射（存在 `~/.config/lark-switch/config.json`）。要以某账号行事时，`lark-switch` 就带着指向该目录的 `LARKSUITE_CLI_CONFIG_DIR` 去运行 `lark-cli`。

> **默认账号很特殊。** 目录为 `~/.lark-cli`（lark-cli 内置默认）的那个账号**绝不能**设 `LARKSUITE_CLI_CONFIG_DIR`：它的加密 token 存在 `~/Library/Application Support/lark-cli`，不在 `~/.lark-cli`，所以钉这个变量反而会让它看起来像未登录。`lark-switch` 会识别并对它留空该变量（`use` 会输出 `unset …`）。这样你就能**零迁移**地直接收编现有已登录账号。

选账号有两种方式：

- **`run`（无状态、并行安全、Agent 友好）**——只为一条命令设变量：`lark-switch run B -- im +chat-list`，不动任何全局状态。
- **`use`（按 shell 切换、gvm 风格）**——需要 shell shim；`lk use B` 只改*当前* shell。编译型二进制改不了父 shell，所以（像 gvm/nvm/direnv 一样）`use` 打印 `export …` 交给 shell 函数 `eval`——见 [`shellenv`](#shell-集成)。在 shim 之外运行（脚本或 Agent 工具调用）时，`use` 不会假装成功：它会打印正确做法并以 exit 1 退出。

## 安装

### 让 Agent 帮你装（推荐）

lark-switch 是给 Agent 用的，最省事的安装方式就是让 Agent 自己装。把下面这段话发给你的 Agent（Claude Code、Codex 等）：

> 安装 lark-switch：从 `https://github.com/xz1220/lark-switch` 运行它的 `install.sh` 把二进制装到 PATH，再把 `SKILL.md` 装进当前 Agent 的 skills 目录。装完确认 `lark-switch ls` 可用，并提醒我重启会话。

Agent 会拉取对应平台的预编译二进制、装好 skill，之后在合适的时机自动调用 lark-switch。

Claude Code 也可以直接用 Marketplace 装 skill/plugin：

```bash
/plugin marketplace add xz1220/lark-switch
/plugin install lark-switch
```

Agent 该遵守的操作规则见 [`SKILL.md`](SKILL.md) 和 [`references/agent-playbook.md`](references/agent-playbook.md)。

### 手动安装

一行命令——从最新 [Release](https://github.com/xz1220/lark-switch/releases) 下载对应 OS/架构的预编译二进制到 `~/.local/bin`：

```sh
curl -fsSL https://raw.githubusercontent.com/xz1220/lark-switch/main/install.sh | sh
```

可用 `LARK_SWITCH_VERSION=v0.3.0` 或 `LARK_SWITCH_BIN_DIR=/usr/local/bin` 覆盖。

<details>
<summary>从源码构建（需 Go 1.25+）</summary>

```sh
git clone https://github.com/xz1220/lark-switch.git
cd lark-switch/src
go build -o lark-switch .
cp lark-switch ~/.local/bin/   # 或 PATH 上任意目录
```

</details>

### Shell 集成

加进 `~/.zshrc`（或 `~/.bashrc`）：

```sh
eval "$(lark-switch shellenv)"
```

这会定义一个 `lk` 函数：`lk use B` 切换当前 shell；其余（`lk ls`、`lk run A -- …`）都原样透传给 `lark-switch`。

## 快速上手（两个账号，不同租户）

```sh
# 1. 把现有已登录账号收编为默认目录（无需重新登录）：
lark-switch add A --dir ~/.lark-cli --note "我 / 租户 A"

# 2. 在独立目录里加第二个账号并登录。
#    （先在账号 B 的飞书管理后台建一个自建应用，备好 App ID + App Secret；
#     --init 会跑 config init + auth login）
lark-switch add B --init

# 3. 查看两个账号：
lark-switch ls
```

日常使用：

```sh
lk use B                       # 当前 shell 切到账号 B
lark-cli calendar +agenda      # …以 B 执行

lark-switch run A -- mail +list  # 一次性以 A 执行，不离开 B
lark-switch each -- auth status  # 对所有账号跑同一条命令
```

## 并行 Claude Code / Agent 会话

启动会话时带上环境变量，就能把整个会话绑定到某个账号（默认账号无需变量）：

```sh
claude                                                   # 会话用账号 A（默认）
LARKSUITE_CLI_CONFIG_DIR=$(lark-switch path B) claude    # 会话用账号 B
```

会话内每次 `lark-cli` 调用（以及每个 `lark-*` skill）都固定到正确账号。Agent 优先用这种方式（或 `run`）而非 `use`——`use`/`profile use` 会改共享状态，惊扰并行会话。

机器可读接口：`ls --json` / `current --json`。Agent 完整规则见 [`SKILL.md`](SKILL.md) 与 [`references/agent-playbook.md`](references/agent-playbook.md)。

## Agent 操作多账号实践（简版）

最稳的流程是**先查清账号状态，再把每条 `lark-cli` 命令固定到某个账号执行**：

```sh
lark-switch ls --json                       # 1. 看有哪些账号、谁是默认
lark-switch run dangdang -- whoami          # 2. 敏感操作前先验证身份
lark-switch run dangdang -- im +chat-list --as user --types group   # 3. 每条命令绑定账号
```

在 Agent 工具调用里**不要**用 `lark-switch use`（它需要交互 shell 的 shim，在工具调用里会直接报错退出），也不要假设账号切换会跨命令继承。完整实践见 [`references/agent-playbook.md`](references/agent-playbook.md)。

## 保活 token

`lark-cli` 的用户 token 用的是**滚动 ~7 天**的续期窗口，每次成功调用都会自动延长；如果一个账号超过 7 天没动过，就得重新登录（扫码 / 设备流）。用 cron 保活：

```cron
# 每天 09:00 给每个账号续期
0 9 * * *  /Users/you/.local/bin/lark-switch refresh --all >/dev/null 2>&1
```

## 命令

| 命令 | 作用 |
|---|---|
| `add <name> [--dir <path>] [--init] [--domain ...]` | 注册账号；`--init` 跑 `config init` + `auth login` |
| `login <name> [--domain ...\|--scope ...] [--init]` | （重新）授权账号 |
| `ls [--json]` | 列出账号、用户、token 状态、续期窗口（当前标 `*`） |
| `use <name>` | 切换当前 shell（需 shim；shim 外 exit 1） |
| `run <name> -- <args>` | 以 `<name>` 执行一条 `lark-cli` 命令 |
| `each -- <args>` | 对所有账号执行一条命令 |
| `refresh [<name>\|--all]` | 保活 token |
| `current [--json]` / `which` | 显示当前 shell 生效的账号 |
| `path [<name>]` | 打印账号配置目录 |
| `rm <name> [--purge]` | 注销（`--purge` 连目录一起删） |
| `shellenv` | 打印 shell shim |

## 说明

- 零依赖（仅 Go 标准库）。面向 macOS（默认目录路径用了 `~/Library/Application Support`），但机制在任何能跑 `lark-cli` 的地方都成立。
- 遵守 `NO_COLOR`；管道输出为纯文本。
- 开发本仓库见 [AGENTS.md](AGENTS.md)。
