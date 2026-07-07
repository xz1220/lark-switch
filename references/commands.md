# 命令参考

全部子命令与 flag。约定：`<name>` 是账号名；`-- ` 之后的内容原样透传给 `lark-cli`。

## 账号选择

### `run <name> [--] <lark-cli args...>`
以 `<name>` 执行**一条** lark-cli 命令，无全局状态改动，并行安全。Agent 首选。底层用 `syscall.Exec` 替换进程，退出码和信号原样透传。

```sh
lark-switch run B -- im +chat-list --as user --types group
```

### `each [--] <lark-cli args...>`
对**所有**已注册账号各跑一遍同样的命令，逐个打印分隔标题。任一账号失败则整体返回非零。

```sh
lark-switch each -- auth status
```

### `use <name>`
切换**当前 shell**（gvm 风格），需要 shell shim（见 `shellenv`）。编译型二进制改不了父 shell，所以 `use` 把 `export ...` 打到 stdout 由 shim `eval`。
**在 shim 之外运行（脚本 / Agent 工具调用）会打印正确替代方案并 exit 1，绝不假装切换成功。** Agent 不要用它。

## 查询

### `ls [--json]`
列出账号：当前账号标 `*`、用户名、token 状态、剩余续期窗口、配置目录。`--json` 输出机器可读结构（见 `json-output.md`）。

### `current [--json]` （别名 `which`）
显示当前进程环境对应的账号（由 `LARK_SWITCH_CURRENT` 或 `LARKSUITE_CLI_CONFIG_DIR` 推断）。未注册时显示 `(unregistered)` 加目录。`--json` 输出 `{name, dir}`（未注册时 `name` 为 `null`）。

### `path [<name>]`
打印账号配置目录的**绝对路径**（省略 `<name>` 则用当前账号）。适合拼进 `LARKSUITE_CLI_CONFIG_DIR=...`。

## 账号管理

### `add <name> [--dir <path>] [--init] [--brand feishu|lark] [--note ...] [--domain ...]`
注册账号。默认目录 `~/.lark-cli-<name>`；`--dir ~/.lark-cli` 可收编现有默认账号（零迁移）。`--init` 接着跑 `config init --new` + `auth login`（交互，需扫码 / 浏览器授权），`--domain` 指定授权范围。

```sh
lark-switch add A --dir ~/.lark-cli --note "me / tenant A"   # 收编现有默认账号
lark-switch add B --init                                     # 新目录 + 登录
```

### `login <name> [--domain ...|--scope ...] [--init]`
重新授权账号。`--scope` 优先于 `--domain`；`--init` 会先跑 `config init --new`。

### `refresh [<name>|--all]`
做一次廉价的已认证调用，让 lark-cli 自动续期滚动的 ~7 天 refresh token。省略名字或 `--all` 处理全部。建议 cron：

```cron
0 9 * * *  /Users/you/go/bin/lark-switch refresh --all >/dev/null 2>&1
```

### `rm <name> [--purge]`
注销账号。默认保留配置目录，`--purge` 连目录一起删（对默认账号 `~/.lark-cli` 会拒绝 `--purge`）。

## 其他

### `shellenv`
打印 shell shim。装法：`eval "$(lark-switch shellenv)"` 加进 `~/.zshrc` / `~/.bashrc`。shim 定义 `lark-switch` 函数（`lk` 为别名），并在调 `use` 时设 `LARK_SWITCH_EVAL=1` 让二进制知道 stdout 确实会被 eval。

### `version` / `help`

## 环境变量

| 变量 | 含义 |
|---|---|
| `LARKSUITE_CLI_CONFIG_DIR` | lark-cli 的配置目录；lark-switch 据此定位账号（默认账号不设） |
| `LARK_SWITCH_CURRENT` | 当前账号名，由 shim 在 `use` 时导出 |
| `LARK_SWITCH_EVAL` | shim 设为 `1`，告诉二进制 `use` 的 stdout 会被 eval |
| `LARK_SWITCH_CONFIG` | 覆盖注册表路径（默认 `~/.config/lark-switch/config.json`） |
| `NO_COLOR` | 非空则输出纯文本 |
