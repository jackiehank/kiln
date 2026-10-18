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

## collect：CLI 怎么暴露「多种关系参数」，以及帮助分组（2026-10-18）

> 起因（用户）：本 issue 不只是 `-h` 分组——关系有好几类，`do`/`commit` 该不该给关系参数做**分组 / 专门设计**，也一并想。先看同行。

### 0. 本仓现状（`relations-impl` 落地后）

- `do`/`commit` 的 flag **已经是从词表生成**的（`e.rel().Names()`）——新名 `--causes` / `--caused-by` / `--part-of` … 已可用，不再是 5 个硬编码。
- 但 `-h` 的 `usageFooter` 仍是**静态字符串**（手工列了 10 个关系 flag），**与配置可能脱节**。
- → 本 issue 现在只剩两问：**帮助怎么组织** + **参数形态要不要改**。

### 1. 业内暴露关系的三种形态

| 形态 | 代表 | 例子 | 优点 | 代价 |
| --- | --- | --- | --- | --- |
| **每类一个 flag**（flat） | `gh issue` v2.94.0 | `--parent` / `--set-parent` / `--remove-parent`；`--blocked-by` / `--blocking` / `--add-*` / `--remove-*` | 可发现、有 shell 补全 | flag 数随词表增长；帮助臃肿 |
| **通用带名 flag** | `git commit --trailer`；Jira CLI `--link`；`--type` | `--trailer "<token>[(=|:)<value>]"`（可重复）；`jira --action linkIssue --link "Relates"` | **与词表解耦**：自定义名直接可用 | 无逐名发现 / 补全；需自己校验 |
| **子命令** | `jira issue link`；`jtk links create/list/delete/types` | `jira issue link A --to B --type Blocks` | 命名空间干净 | 与 kiln 的动作提交模型不合（关系得写进历史） |

### 2. 帮助分组是成熟做法

- **Cobra flag groups**：用 flag annotation 分组，帮助按组渲染（例：`entireio/cli` 把 flag 分成 Scope / Navigation / Filtering & Sorting / Formatting，且规定“Scope 永远最先”）。— 一手。
- GitHub CLI 的帮助也按主题组织。

### 3. 三条对 kiln 的观察（不下结论）

1. **子命令不合适**：kiln 的关系是**动作提交里的 trailer**（写一处、读侧推导）；`kiln relate ...` 要么得写进历史、要么变成第二条边，与 `relation-revision` 的**单值槽**冲突。→ 参数仍应挂在 `do`/`commit` 上。
2. **通用 `--relate name=target` 值得考虑**：它就是 `git --trailer` 的形态，且**天然适配“词表可配置”**（自定义名不用改 CLI）；代价是可发现性差、要额外校验。
3. **flat 逐名 flag + 帮助分组**：保留 `--causes` 这类逐名 flag（可发现 / 可补全），但在 `-h` 里给它们一个**独立小节**（Cobra 式分组），并**从词表动态生成**；`unrelate` 放进关系小节。

→ 可能的组合：**flat 逐名 flag（词表生成）+ 通用 `--relate`（逃生口）+ `-h` 关系小节（动态）**。

### 4. 来源

- `git commit --trailer <token>[(=|:)<value>]`（可重复、自由 token）：[git-commit](https://git-scm.com/docs/git-commit)（一手）。
- GitHub CLI v2.94.0 的 issue 层级 / 依赖 flag：`gh issue --parent / --blocked-by / --blocking`（[GitHub Changelog](https://github.blog/changelog/2026-06-10-manage-sub-issues-types-and-dependencies-from-github-cli/)）（一手）。
- Jira CLI 通用 `--link`：`jira --action linkIssue --issue X --toIssue Y --link "Relates"`（[Appfire JCLI linkIssue](https://appfire.atlassian.net/wiki/spaces/SUPPORT/pages/89147359/How+to+use+linkIssue)）（一手 / 厂商文档）。
- 子命令式关系管理：`jira issue link`（[matcra587/jira-cli](https://pkg.go.dev/github.com/matcra587/jira-cli@v0.9.1/internal/cli/issue)）、`jtk links create ... --type`（[atlassian-cli #148](https://github.com/open-cli-collective/atlassian-cli/issues/148)）（二手 / 提案）。
- Cobra flag groups： [entireio/cli flaggroups.go](https://github.com/entireio/cli/blob/v0.11.3/cmd/entire/cli/flaggroups.go)（一手）。
