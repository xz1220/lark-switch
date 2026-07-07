# JSON 输出结构

供程序化解析。所有 JSON 走 stdout；错误走 stderr 并 exit 1。`NO_COLOR` 与非 TTY 输出均为纯文本。

## `ls --json`

```json
{
  "current": "A",
  "accounts": [
    {
      "name": "A",
      "dir": "/Users/you/.lark-cli",
      "current": true,
      "default_home": true,
      "brand": "feishu",
      "note": "me / tenant A",
      "user": "邢政",
      "open_id": "ou_6f95...",
      "status": "valid",
      "refresh_expires_at": "2026-07-14T20:41:19+08:00",
      "refresh_in_seconds": 599159
    }
  ]
}
```

字段：

| 字段 | 类型 | 说明 |
|---|---|---|
| `current` | string | 当前进程环境对应的账号名；无则省略 |
| `name` | string | 账号名 |
| `dir` | string | 配置目录绝对路径 |
| `current` | bool | 是否当前账号 |
| `default_home` | bool | 是否 lark-cli 默认目录 `~/.lark-cli`；**为 true 时不要手动设 `LARKSUITE_CLI_CONFIG_DIR`** |
| `brand` | string | `feishu` / `lark`（可能缺省） |
| `note` | string | 备注（可能缺省） |
| `user` | string | 用户名；仅 bot 授权时缺省（表格里显示 `(bot only)`） |
| `open_id` | string | 用户 open_id（可能缺省） |
| `status` | string | token 状态：`valid` / `needs_refresh` / `not-configured` / `error` / `lark-cli-not-found` / `unknown` |
| `refresh_expires_at` | string | refresh token 到期时间 RFC3339（可能缺省） |
| `refresh_in_seconds` | int | 距到期秒数；负数表示已过期（可能缺省） |

无账号时：`{"accounts": []}`。

## `current --json`

```json
{"name": "A", "dir": "/Users/you/.lark-cli"}
```

当前目录未映射到任何已注册账号时，`name` 为 `null`，`dir` 是 `LARKSUITE_CLI_CONFIG_DIR`（未设则为默认目录）：

```json
{"name": null, "dir": "/tmp/nope"}
```

## `path <name>`

非 JSON，单行绝对路径（便于直接拼进环境变量）：

```
/Users/you/.lark-cli-dangdang
```

## 解析建议

- 判断「是否需要重新登录」：`status != "valid"` 或 `refresh_in_seconds` 偏小 / 为负。
- 选账号目录：优先用 `path <name>`，或读 `ls --json` 里对应 `dir`。
- 判断默认账号用 `default_home` 布尔，不要靠字符串比对路径。
