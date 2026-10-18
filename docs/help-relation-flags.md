# issue: help-relation-flags — `-h` 里关系 flag 与普通 flag 混排，且硬编码

- **domain**：cli
- **关联**：derived-from `relation-revision`、relates-to `help-alignment` / `help-self-contained`

## 背景

`kiln -h` 的 `do/commit 的 flags:` 一段把两类东西混在一起：

```
  --title  --refs  --target  --scope  --domain  --tag
  --derived-from  --spawns  --relates-to  --blocks  --supersedes
```

- 第一行是**元数据 / 归类 flag**（标题、引用、目标、范围、domain、tag）；
- 第二行是**关系 flag**，而且这 5 个名是**硬编码**在 `usageFooter`（`internal/cli/cli.go:34-36`）里的静态字符串。

两个问题叠在一起：

1. **分组不清**：关系 flag 没有自己的小节 / 说明，读者看不出它和前一行是两类。
2. **硬编码脱节**：关系词表已定要**可配置**（`relation-revision`），定稿名是 `causes` / `caused-by` 等，还多了 `unrelate`；`-h` 若仍列旧的 5 个名，就会显示不存在的能力、并漏掉新能力。

## 要回答的问题

1. **怎么分组**：`do/commit 的 flags` 是否拆成「通用 flag」+「关系 flag」两段？关系段要不要一句说明（如"关系：写一侧、读侧推导反向"）？
2. **动态还是静态**：关系 flag 列表是否从 `relations.names`（配置）**生成**？若是，`usageText()` 需要拿到配置。
3. **`unrelate` 放哪**：tombstone 作为关系段的一员，还是单独一行（“撤销关系”）？
4. **与其它帮助内容的一致性**：action 词表已经是**从配置读**的（`-h` 里 "action 词表默认: …"）——关系段应否照同一模式？
5. **自定义词表**：项目自定义关系名不同时，`-h` 是否反映？

## 候选（草案）

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只分组 | 关系 flag 挪到独立小节，仍静态 | 最小；仍与配置脱节 |
| **B** 分组 + 动态 | 关系段从 `relations.names` 生成 | 要打通配置到 `usageText` |
| **C** 不列具体名 | 只写"关系 flag：见 `relations` 配置 / `kiln config`" | 帮助不会过期，但可发现性差 |

## 非目标

- 不改关系机制 / 词表本身（归 `relation-revision` / `relation-vocabulary`）。
- 不改 `-h` 其它段落（除非为一致性）——那是 `help-alignment` 的地盘。

## 证据

- `internal/cli/cli.go:34-36`：`usageFooter` 硬编码 5 个关系 flag。
- `internal/cli/cli.go:323-327`（flag 解析）、`:1282-1286`（摘要输出）：同源硬编码。
- `relation-revision` 的 test（2026-10-18）：新名 `causes:` / `unrelate:` 当前被**静默丢弃**；词表将可配置。
