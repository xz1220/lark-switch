# Agent 多账号操作实践

给同一个飞书文档 / 群聊处理不同账号、不同租户权限时，最稳定的流程是**先把账号状态查清楚，再把每条 `lark-cli` 命令固定到某个账号上执行**。本文件是 `SKILL.md` 铁律的展开版。

## 为什么必须逐条绑定

Agent（Claude Code、Codex 等）的每个工具调用都是独立子进程。子进程改不了父进程的环境，环境也不会从上一条命令带到下一条。因此：

- `lark-switch use <name>` 在工具调用里**没有意义**——它只把 `export ...` 打印到 stdout，需要交互 shell 的 shim 去 `eval` 才生效；shim 之外它会直接报错退出（exit 1），提示你改用 `run`。
- 即便某条命令临时设了环境变量，下一条工具调用也不会继承。

结论：**每条关键命令自带账号选择**，二选一：

```sh
lark-switch run <name> -- <lark-cli 参数>
# 或
LARKSUITE_CLI_CONFIG_DIR="$(lark-switch path <name>)" lark-cli <参数>
```

## 第一步：查清账号

```sh
lark-switch ls --json
```

关注 `default_home`（是否默认账号）、`status`（token 是否 valid）、`refresh_in_seconds`（多久后需重新登录）。字段结构见 `json-output.md`。

## 第二步：验证身份

改权限、加成员、发消息这类敏感 / 不可逆操作前，先确认命令真的跑在目标账号上：

```sh
lark-cli whoami                    # 当前默认身份
lark-switch run dangdang -- whoami # dangdang 账号的身份
```

## 第三步：逐条执行

一次性调用优先用 `run`：

```sh
lark-switch run dangdang -- im +chat-list --as user --types group --sort active_time
lark-switch run dangdang -- im chat.members get --chat-id oc_xxx --member-id-type open_id --page-all
lark-switch run dangdang -- drive +member-add --token "https://xxx.feishu.cn/wiki/..." \
  --member-id "ou_a,ou_b" --member-type openid --perm full_access --perm-type container --yes
```

连续多条、想少打字时，用 `path` 取目录再显式带前缀：

```sh
DIR="$(lark-switch path dangdang)"
LARKSUITE_CLI_CONFIG_DIR="$DIR" lark-cli whoami
LARKSUITE_CLI_CONFIG_DIR="$DIR" lark-cli im +chat-list --as user --types group
```

## 默认账号（`~/.lark-cli`）的坑

默认账号的加密 token 存在 `~/Library/Application Support/lark-cli`，**不在** `~/.lark-cli`。所以手动把 `LARKSUITE_CLI_CONFIG_DIR` 指向 `~/.lark-cli` 会让它看起来「未登录」。

- 不要对默认账号手动设 `LARKSUITE_CLI_CONFIG_DIR`。
- 通过 `lark-switch run <name> -- ...` 进入即可——`run` 对默认账号会**故意不设**这个变量，对其他账号才设。
- `ls --json` 里 `"default_home": true` 就是这个账号。

## 会话级绑定（可选）

如果整个 Agent 会话只服务一个非默认账号，可以在启动会话时就绑定：

```sh
LARKSUITE_CLI_CONFIG_DIR=$(lark-switch path B) claude   # 本会话内所有 lark-cli 都是 B
```

会话内每次 `lark-cli` 调用（以及每个 `lark-*` skill）都固定到该账号。默认账号无需设变量，直接启动即可。这比 `use` 更适合 Agent——`use`/`profile use` 会改共享状态、惊扰并行会话。

## 和 lark-* skill 配合

`lark-*` skill 都是对 `lark-cli` 的封装，默认跑当前默认账号。要换账号时，把 skill 输出的 `lark-cli <args>` 包一层：

```
lark-cli im +chat-list ...   →   lark-switch run <name> -- im +chat-list ...
```

或给该 skill 的命令加 `LARKSUITE_CLI_CONFIG_DIR` 前缀。

## 速查

| 场景 | 命令 |
|---|---|
| 有哪些账号 | `lark-switch ls --json` |
| 验证身份 | `lark-switch run <name> -- whoami` |
| 单条操作 | `lark-switch run <name> -- <args>` |
| 连续操作 | `LARKSUITE_CLI_CONFIG_DIR="$(lark-switch path <name>)" lark-cli <args>` |
| 整会话绑定 | 启动时设 `LARKSUITE_CLI_CONFIG_DIR=$(lark-switch path <name>)` |
| ❌ 禁用 | `lark-switch use` / `lark-cli profile use` |
