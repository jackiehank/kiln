# issue: action-view — 给“全局 action 索引”定命令名与入口

- **domain**：cli
- **关联**：relates-to `timeline-stream`（上一轮 conclude：需要 action 级全局视图）；relates-to `machine-readable-output`（NDJSON / 字段契约）；relates-to `path-batch`（批量读取先例）

## 背景

`timeline-stream` 已 conclude：kiln 需要一个 action 粒度的全局视图；它不是 `git log` 包装，而是 `list`（issue 索引）在 action 维度上的对称物。上一轮只定了“要这个视图”，没有定命令名与入口。

用户倾向 `action list` 之类的命名；本 issue 先收集类似 CLI 的子命令命名方式与 kiln 已有命令，再 hypothesize 选名，最后 apply。

## 要定的问题

1. **命令形态**：`kiln action list`（名词组 + 动词）、`kiln actions`（复数名词组）、`kiln log`（顶层动词）、`kiln timeline`（视图名）还是别的？
2. **单复数**：kiln 现有名词组用单数（`worktree`、`config`）；`actions` 复数是否值得为了短而破例？
3. **动词**：`list` 还是 `ls`？kiln 已有命令全用全词；`gh` / `docker` 的 `ls` 是别名而非主名。
4. **分组边界**：如果叫 `kiln action list`，`action` 组以后会不会长出 `show` / `search`？它们与现有 `show <issue> n`、`list --search` 的边界是什么？
5. **与 `list` 的区分**：顶层 `list` 是 issue 列表；`action list` 是动作列表。名字要能一眼区分，而不是让 `list` 兼职。
6. **交付契约**：`--jsonl`、`--limit`、字段、NDJSON 例外、jj、broken pipe / 空仓——留给后续 apply；本 issue 只定名与入口。

## 非目标

- 不改代码；不决定“现在做还是等 webui/tui”；
- 不新增模型字段 / 校验；
- 不讨论 `path --timeline` 之外的入口（`timeline-stream` 已排除 `path` 模式）。

## collect（2026-10-28）：CLI 子命令命名惯例与 kiln 现有命令面

> 本节只收事实与候选，不选名；命名裁决留给 hypothesize。

### 1. kiln 现在的命令面

| 形态 | 例子 | 说明 |
| --- | --- | --- |
| 顶层动词 | `do` / `commit` / `path` / `show` / `status` / `check` / `list` / `graph` | 一个词就是一个操作；`list` 已经是 **issue 列表** |
| 名词组 + 动词 | `worktree new|list|remove` | 单数名词 + 全词动词；`list` 是子命令 |
| 名词组 + 单子命令 | `config path` | 组里只有一个子命令也可接受 |
| 无复数组、无 `ls` | — | 现有命令全用全词，没有 `ls` / `ps` 这类缩写 |

要点：

- `list` 被顶层 issue 列表占用；如果 action 视图也叫 `list`，会直接歧义。
- kiln 的名词组用**单数**（`worktree`、`config`），不是复数。
- `worktree list` 证明“`list` 作为子命令”在 kiln 里已有先例。
- `config path` 证明“名词组里只有一个子命令”不是问题。

### 2. 相近 CLI 的两种主形态

| 工具 | 例子 | 形态 | 备注 |
| --- | --- | --- | --- |
| git | `git log` | 顶层动词 | 全局提交历史 |
| git | `git worktree add|list|lock|move|prune|remove|repair|unlock` | **单数名词 + 动词** | `list` 是子命令；资源是一等对象 |
| gh | `gh issue list` / `gh pr list` / `gh run list` | **单数名词 + `list`** | `gh issue ls` 是别名；`--json` / `--limit` 成熟 |
| jj | `jj log` | 顶层动词 | 全局 revision 历史 |
| jj | `jj workspace list` | **单数名词 + `list`** | 与 `jj log` 并存 |
| docker | `docker container ls` | **单数名词 + 短动词** | 别名 `list` / `ps`；旧式 `docker ps` 仍可用 |
| kubectl | `kubectl get pods` | **动词 + 复数资源参数** | 不是名词组；资源是参数 |
| npm | `npm ls` | 顶层短动词 | `npm ls` 是常用短名（与 `npm list` 同义） |

两种主形态都成立：

1. **名词组 + 动词**：当名词是一等资源、且有多个操作时（git `worktree`、gh `issue`、jj `workspace`、docker `container`）。
2. **顶层动词**：当操作本身就是全局的（git/jj `log`、kubectl `get`、npm `ls`）。

### 3. 六条可迁移观察

1. **单数是名词组的主流**：`worktree` / `issue` / `workspace` / `container` 都是单数；复数多出现在参数（`kubectl get pods`）或旧式别名（`docker ps`）。→ `action list` 比 `actions` 更贴近惯例。
2. **`list` 是名词组里的常见主名**：git / gh / jj 都用全词 `list`；docker 用 `ls` 但同时提供 `list` 别名。kiln 全用全词，→ `action list` 比 `action ls` 更一致。
3. **`ls` / `ps` 是历史或短化选择**，不是 kiln 的现有风格；引入 `action ls` 会开一个“何时用缩写”的新口子。
4. **全局历史有“顶层动词”传统**（`git log` / `jj log`），但那两个工具没有“issue/action”资源组；kiln 已有 `worktree` 组，动作也是一等资源，→ 名词组并不突兀。
5. **`action` 是模型词表里的词**：命令名复用 `action` 能让“模型词 → 命令名”一一对应；`timeline` 是 UI 比喻，`log` 是 git 的词，都会多一层翻译。
6. **名词组会暗示后续子命令**：`action list` 一旦成立，用户会期待 `action show` / `action search`。要么在 hypothesize 里明确“当前只开 list，show/search 仍走现有命令”，要么接受组会生长。`config path` 的先例说明“组里只有一个子命令”可以接受，但 `action` 比 `config` 更容易让人期待更多。

### 4. 候选名

| 候选 | 形态 | 参照 | 优点 | 风险 |
| --- | --- | --- | --- | --- |
| **`kiln action list`** | 单数名词 + `list` | git `worktree list`、gh `issue list`、jj `workspace list` | 资源明确；与顶层 `list` 区分；单数/全词都合 kiln 惯例；给 `--jsonl` / `--limit` 留位置 | 三个 token；组里目前只有一个子命令；会暗示 `action show` |
| `kiln actions` | 复数名词组 | docker `ps` 的旧式短名 | 短 | 复数破例；无动词，默认行为不明（是 list 还是 help？） |
| `kiln action log` | 单数名词 + `log` | `git log` | 保留“日志”语义 | `log` 与 `git log`、已否的 `log --rewrites` 撞；`action log` 像审计 |
| `kiln timeline` | 视图名 | 无直接资源组对照 | 直接说“时间线”；不假装成资源 | 不是资源名；与 kiln 的 `worktree` / `config` 组风格不同 |
| `kiln log` | 顶层动词 | git/jj `log` | 最短；与 git 习惯一致 | 撞 `git log` 与 `log --rewrites`；`path` 已是单 issue 的 log 生态位 |
| `kiln action ls` | 单数名词 + 短动词 | docker `container ls`、npm `ls` | 更短 | kiln 现有命令全用全词；开缩写口子 |

### 5. 留给 hypothesize 的问题

1. 选 `action list`（名词组 + 全词）还是 `timeline` / `log`（顶层动词/视图名）？
2. 如果选 `action list`：`action` 组当前只开 `list`，还是同时预留 `action show` / `action search`？后者与 `show <issue> n`、`list --search` 的边界是什么？
3. `action list` 的默认输出是文本表格还是 NDJSON？`--jsonl` 是否必填？`--limit` 是否只跟它？
4. 是否接受“名字里带 action、但输出的是全局动作流”这一层语义？还是更愿意用 `timeline` 直接说视图？
5. 是否引入别名（`als` / `al` / `action ls`）？kiln 现有命令没有别名传统，默认不加。

### 来源

- 本机命令面：`kiln -h`、`kiln worktree -h`、`git worktree -h`、`jj log -h`、`jj workspace list -h`、`docker container ls --help`、`kubectl get -h`、`npm ls --help`（2026-10-28）。
- [git worktree](https://git-scm.com/docs/git-worktree)
- [gh issue list](https://cli.github.com/manual/gh_issue_list)
- [jj CLI reference](https://docs.jj-vcs.dev/latest/cli-reference/)
- [Docker container ls](https://docs.docker.com/reference/cli/docker/container/ls/)
- [kubectl reference](https://kubernetes.io/docs/reference/kubectl/)
- [npm ls](https://docs.npmjs.com/cli/v10/commands/npm-ls)

## hypothesize（2026-10-28）：选 `kiln action list`——单数名词 + 全词 `list`，只开 `list`

> 结论先行：`action list` 是 collect 里唯一同时满足“单数名词组 / 全词 / 与顶层 `list` 区分 / 复用模型词”的候选，选它。命令形态为 **`kiln action list [--jsonl] [--limit N]`**：默认给人读的一行一条，`--jsonl` 给机器流；不需要 `--all`；当前**只开 `list`**，不做 `action show` / `action search`。主要风险是“action list”可能被读成“列出 action 词表”，用 help 文案和一次用户测试钉住；若读错严重，退回 `timeline`。

### 1. 主张

```text
kiln action list [--jsonl] [--limit N]
```

- `action` 单数、`list` 全词：与 `worktree list`、`gh issue list`、`jj workspace list` 同形；与顶层 issue `list` 区分。
- 全局：不带 `--all`，也不接受 issue 参数；时间顺序 author-date 倒序。
- 默认文本：一行一条动作，含日期 / issue / action / 短 hash / subject；具体列宽 apply 定。
- `--jsonl`：NDJSON，一行一个动作对象，字段与 `timeline-stream` 定的 action 视图一致。
- `--limit N`：最多 N 条动作后正常退出；只跟 `action list`；N ≥ 1。
- 当前**只开 `list`**：`show <issue> n` 仍是动作详情，`list --action` 仍是 issue 过滤；不新增 `action show` / `action search`。

### 2. 为什么是 `action list`

| 候选 | 判定 | 理由 |
| --- | --- | --- |
| **`action list`** | **选** | 单数名词组、全词 `list`、复用模型词 `action`、与顶层 `list` 区分；`config path` 证明单子命令组可接受 |
| `actions` | 不选 | 复数破 kiln 名词组惯例；没有动词，默认行为（list？help？）不明确 |
| `action log` / `log` | 不选 | `log` 撞 `git log` 与已否的 `log --rewrites`；`action log` 容易被读成审计 |
| `timeline` | 备选 | 直接说视图，但不是资源名；与 `worktree` / `config` 组风格不同；作为 fallback 保留 |
| `action ls` | 不选 | kiln 现有命令全用全词；`ls` 是 docker/npm 的短化传统，不是 kiln 的 |

`action` 是 trailer 键、模型词、`show` 的选择器；命令名复用它，比 `timeline` / `log` 少一层翻译。

### 3. 默认文本 + `--jsonl` 的产品形态

- **默认文本**：`action list` 是 `list` 的 action 版，应该能直接给人看；一行一条，流式打印，不建全史。
- **`--jsonl`**：机器契约，NDJSON；`action list` 是唯一带 NDJSON 的命令，作为“一份文档”规则的显式例外。
- **不提供 `--json`**：一份 action 数组会和 `--jsonl` 争同一个视图，还要先缓冲；本轮不做。
- **`--limit` 对两种输出都有效**：到 N 条动作就停，不是到 N 个提交。
- **流式**：文本与 NDJSON 都是边扫边出；首行不等全仓。

### 4. 边界与不做什么

- 不做 `action show`：动作详情仍走 `show <issue> n`；将来若加，应先解决“全局按 commit 寻址”与现有 issue+ordinal 的关系。
- 不做 `action search`：搜索仍走 `list --search`；动作流的过滤留给消费方或后续 issue。
- 不做 `--domain` / `--tag` / `--action` 过滤：本轮只做“全部动作的时间线”；要过滤先在消费方做，或另开 issue。
- 不做 `--all`、`--stat`、`--shortstat`、`--json`；`action list` 不是 `path` 的模式。
- 不支持 jj：`action list` 在 jj 后端显式非 0 + stderr「暂不支持」，不静默空输出；等 jj 的流式/排序方案再补。
- 空仓 / 无动作：stdout 为空、exit 0（“没有动作”不是错误）；stderr 是否给提示在 apply 定。
- broken pipe：及时停止扫描、结束 git 子进程，不挂住；退出码优先 0，具体在 apply 定。

### 5. 契约草案（apply 定死）

| 项 | 草案 |
| --- | --- |
| 输出 | 默认文本一行一条；`--jsonl` 一行一个 JSON 对象 |
| 字段 | `issue / action / commit / date / title / domain / tags`（与 `timeline-stream` 一致） |
| 顺序 | author-date 倒序；拓扑约束下的顺序，文档写明 merge 多时不保证严格全局时间序 |
| 过滤 | 只留同时带 `issue:` + `action:` 的提交 |
| limit | N 条动作；N ≥ 1；无效 exit 2 |
| 空/错 | 空仓/无动作 exit 0；读历史失败非 0；jj 明确非 0 |
| NDJSON | `action list --jsonl` 是唯一例外；其他命令仍“一份 JSON 文档” |

### 6. 反方与证伪

**最大风险：`action list` 可能被读成“列出 action 词表”。** kiln 里 `action` 既是“动作实例”，也是“动作类型”（`ask` / `test` / …）；`action list` 听起来像列后者。

- 缓解：help 文案第一句写“按时间列出动作（提交），不是列出动作词表；词表见 `kiln -h` / `.kilnconfig.jsonc`”。
- 证伪：找 3–5 个用过 kiln 的人（或插件侧）只看命令名猜用途；多数猜“词表”就改名。
- 备选顺序：`timeline` → `actions`（若愿意破单数惯例）→ `action log`。

其他证伪条件：

1. **默认文本不必要**：如果所有人都只用 `--jsonl`，`action list` 的“list”就是多余包装，应该用 `timeline --jsonl`。
2. **组会生长**：如果很快要加 `action show` / `action search`，而它们与 `show` / `list --search` 的边界说不清，独立 `timeline` 反而更干净。
3. **jj 必须首日支持**：如果 jj 用户要求等价，`action list` 的“git-only”承诺要重新评估（可先明确拒绝）。
4. **`--jsonl` 必须显式**：如果用户期望 `action list` 默认 NDJSON，文本默认就是错的产品形态；需重新定。

### 7. 下一步

- 若认可 `action list`：本 issue 下一步是 `conclude`（定名、默认文本、只开 list、jj 拒绝、`--jsonl`/`--limit`），然后另开/继续 `apply` 实现。
- 若对“词表歧义”不放心：先做一次极小用户测试或在 plugin 侧确认，再 conclude；不要直接 apply。
- 实现前仍需定死：文本列宽/日期格式、`--limit` 与 broken pipe、空仓提示、jj 错误文案、NDJSON 字段的 `title`/`domain` 语义。
