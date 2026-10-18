# issue: config-compat — 配置的版本兼容：老配置 / 新配置 / 拼错键

- **domain**：cli
- **关联**：derived-from `help-relation-flags`、relates-to `config-home`

## 背景（实测，2026-10-18）

`-h` 现在会读配置（且**容错**）。用临时仓实测三种配置：

| 配置内容 | `kiln -h` | 普通命令（如 `status`） |
| --- | --- | --- |
| **老式**：只有 `actions` / `vcs` / `worktree` | 正常 | ✓ 正常（缺 `relations` → 默认词表） |
| **带未实现的段**：多了 `"relations": {…}` | 正常（容错、回退默认） | ✗ `配置错误: unknown field "relations"` |
| **拼错键**：`"vcsx": "auto"` | 正常（容错） | ✗ `unknown field "vcsx"`（**有意为之**） |

## 要回答的问题

1. **向前兼容**：老 kiln / 新配置（带 `relations` 等新段）会**硬失败**。要不要容许「未知顶层键」——由**硬错误**改为**⚠ 警告并继续**？（这会背离 `config.go` 现有注释的取向："a misspelled key is never silently ignored"。）
   - **已定（2026-10-18，用户）**：**不考虑向前兼容** → 维持严格解析（方案 A）。只保证「**老配置 → 新 kiln** 兼容」（缺 `relations` 用默认词表），不追求「新配置 → 老 kiln」。
2. **配置版本号**：加 `"version"` 字段？能提示迁移，但对**老 kiln** 无解（它照样因 `relations` 未知而报错）——除非先解决第 1 条。
3. **`relations` 何时进 schema**：`relations-impl` 只把默认词表接进了 `Config`，**文件里的 `relations` 段尚未解析**（设计已定，纯 plumbing）。在此之前，"词表可配置"只在代码里成立，配置里写就报错。
4. **一致性**：`-h` 容错、其它命令严格——这个不一致可接受吗？（好处：坏配置也能看帮助；坏处：坏配置下 `-h` **静默展示默认词表**，可能与项目实际不符。）

## 附带更正（一处前序错误）

- `-h` 的 `action 词表默认: ask | …` 行**仍是静态字符串**——实测自定义 `ask | done` 未反映。
- → `help-relation-flags` 的 collect 里说"action 词表已经是从配置读的"**是错的**；action 词表行与关系小节一样需要**从配置生成**。

## 候选（草案）

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 维持严格 | 未知顶层键 → 硬错误 | 向前不兼容；但拼错可见 |
| **B** 未知键 → ⚠ 并继续 | 兼容未来段；拼错仍可见（警告） | 违反"从不静默忽略"的**硬**取向（但已不静默） |
| **C** 加 `version` + A | 版本不符给迁移提示 | 对老 kiln 无解 |
| **D** B + `version` | 兼容 + 迁移提示 | 同 B |

## 非目标

- 不改配置的**位置 / 格式**（`.kilnconfig.jsonc`、JSONC）——那是 `config-home`。
- 不在本 issue 里落地 `relations` 配置段本身（归 `relations-impl` P0）。

## 证据

- `internal/config/config.go`：`file` 结构体 + `dec.DisallowUnknownFields()`；`Read` 对解析错误直接返回错误。
- `internal/config/config.go` `defaultFile`：播种的默认配置只含 `actions` / `vcs` / `worktree`。
- `help-relation-flags` 的 apply（2026-10-18）：`-h` 改为读配置且容错；`usagePrologue` 里的 action 词表行仍是静态。
