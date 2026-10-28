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

## hypothesize（修订，2026-10-28）：两条命令分开——`action list` 列词表，`timeline` 列动作

> 结论先行：上一节把 `action list` 给了“动作实例”，并把它可能被读成“列出动作词表”列为最大风险。现在这个风险变成需求：**action 可配置，确实需要一条列出可用 action 的命令**。因此把 `action list` 给**词表**，把全局动作实例视图改用 `timeline`。两条命令一起设计，避免用 `action types` / `config actions` 这类发现性更差的名字去绕开歧义。

### 1. 两条命令的分工

| 命令 | 回答的问题 | 单位 | 是否读历史 |
| --- | --- | --- | --- |
| `kiln action list [--json]` | 现在有哪些 action 可用？起点/终点是谁？ | 词表（action 类型） | **否**：只读配置（或默认词表） |
| `kiln timeline [--jsonl] [--limit N]` | 全局按时间发生了哪些动作？ | 动作实例（提交） | 是：流式读 git 历史 |

一句话：**`action list` 是 schema，`timeline` 是 events。**

### 2. `kiln action list`

```text
kiln action list [--json]
```

- 输出生效的动作词表（`.kilnconfig.jsonc` 的 `actions`，没有则内置默认）；
- 文本：一个 action 一行，标注 `start` / `terminal`；不加表头、不加 legend，方便 `| wc -l`；
- `--json`：一份对象，字段尽量复用配置里的词：
  ```json
  {
    "actions": {
      "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
      "start": "ask",
      "terminal": ["conclude", "apply"]
    },
    "source": "file",
    "config": "C:\\Users\\jacki\\kiln\\.kilnconfig.jsonc"
  }
  ```
  - `source`：`file`（读了配置文件）或 `default`（内置词表）；
  - `config`：仅 `source=file` 时出现；默认时省略；
  - `actions` 的形状与 `.kilnconfig.jsonc` 的 `actions` 组同形，便于对照。
- **不需要仓库**：仓外/未初始化时用默认词表，`source=default`；不读 git 历史，不为它开后端。
- 不做 `--jsonl`、不做 `--limit`、不加过滤：词表很小，一份文档足够。
- 与 `do` / `commit` 使用同一个 `ActionSet`：`do` 拒绝非法 action 的判据，和这里列出的集合必须同源。

### 3. `kiln timeline`

```text
kiln timeline [--jsonl] [--limit N]
```

- 全局动作流：只留同时带 `issue:` + `action:` 的提交；按 author-date 倒序；
- 默认文本：一行一条动作，含日期 / issue / action / 短 hash / subject；不做 issue 分组、不做 ordinal/relations/stat；
- `--jsonl`：NDJSON，一行一个动作对象，字段沿用 `timeline-stream` 定的 `issue / action / commit / date / title / domain / tags`；
- `--limit N`：最多 N 条动作后正常退出；N ≥ 1；
- 不需要 `--all`（timeline 天生全局）；不提供 `--json`（一份数组会和 `--jsonl` 争同一个视图）；
- 不支持 jj：显式非 0 + stderr「暂不支持 jj」，不静默空输出；
- 空仓/无动作：stdout 为空、exit 0；broken pipe 及时停扫描、结束 git 子进程。

**文本最小视图的边界**：这不是 `timeline-stream` 里说“本轮不做”的富 timeline 视图；它只是“一行一条、能直接用眼睛看”的最小文本，不做高亮、分组、交互。若连这层也不想要，`timeline` 可以只保留 `--jsonl`；但那样 `timeline` 无 flag 时没有输出形态，产品上更差。

### 4. 为什么不是别的分配

| 方案 | 问题 |
| --- | --- |
| `action list` = 实例，`action types` = 词表 | 保留上一版命名，但 `action list` 仍然最容易被读成词表；`types` 发现性差，等于把歧义藏起来 |
| `action list` = 实例，`config actions` = 词表 | `config` 命名空间语义上合理，但用户不会先想到去 `config` 下找“我能用哪些 action”；`action list` 仍然歧义 |
| `actions` = 词表，`timeline` = 实例 | `actions` 复数破单数惯例，且“the actions”也可能指实例 |
| `action list` = 词表，`action log` / `log` = 实例 | `log` 撞 `git log` 与已否的 `log --rewrites`；`action log` 像审计 |
| **`action list` = 词表，`timeline` = 实例** | **选**：两个名字分别对应“schema / events”，歧义消失；`timeline` 是视图名，像 `status` / `graph` 一样可接受 |

`timeline` 不是资源名，这是它的代价；但它回答的是“按时间看事件”，本来就是视图，不是资源。反过来，`action list` 回答“有哪些类型”，资源/词表语义更强。

### 5. 与现有命令的边界

| 命令 | 单位 | 边界 |
| --- | --- | --- |
| `list` | issue | issue 索引；`--action` 是“出现过某类动作的 issue”，不是动作列表 |
| `path <issue>` | 单 issue 的 path | issue 内的动作序列 + 关系/ordinal |
| `show <issue> n` | 单个动作 | 动作详情；不做全局寻址 |
| `action list` | 词表 | 当前可用的 action 类型；不读历史 |
| `timeline` | 动作实例 | 全局历史；**不按当前词表过滤**——历史里的旧/非法 action 仍会出现，合法性归 `check` |

最后一条很重要：`action list` 是当前 schema，`timeline` 是历史事实。不要因为某个 action 已从配置里删掉，就让 timeline 静默漏掉历史提交。

### 6. 风险与证伪

1. **`timeline` 不是资源名**：如果用户一致期待 `action timeline` / `actions`，说明“视图名”不成立；可把实例命令改名为 `actions`，但 `action list` 继续留给词表。
2. **两条命令是否过度**：`action list` 很小，`timeline` 是另一个视图；如果实践里没人问“有哪些 action”，词表命令可以退化成 `-h` 已经够用，砍掉 `action list`。但 action 可配置后，独立命令的价值是机器可读（`--json`）与脚本可用。
3. **`action list` 与 `-h` 重复**：`-h` 是帮助，不是稳定 JSON 契约；`action list --json` 给 agent/脚本，二者不冲突。
4. **timeline 默认文本是否必要**：如果插件/客户端只用 `--jsonl`，文本是额外实现；但它让命令可被人直接使用，且与 `list` 的文本默认一致。若砍掉文本，`--jsonl` 必须变成必填。
5. **词表过滤**：如果产品要求 timeline 只显示当前合法 action，那是另一个决定；本方案选择“历史事实不因配置变化而消失”，并交给 `check` 报非法。

### 7. 下一步

- 若认可这对命名：`conclude` 定稿“`action list` = 词表、`timeline` = 动作流；`action list` 带 `--json`，`timeline` 默认文本 + `--jsonl` + `--limit`；jj 明确不支持”；实现可拆两个 `apply`（词表命令小，先落）。
- 若仍想在 `action list` 上保留实例语义：需要先回答“词表命令叫什么、为什么用户不会先猜错”，否则不要回到上一版。
- 实现前仍需定死：`action list` 文本的 `start`/`terminal` 标记格式、`source`/`config` 的省略规则；`timeline` 的文本列宽/日期格式、NDJSON 字段、`--limit`/broken pipe、空仓提示、jj 错误文案。

## conclude（2026-10-28）：action 组定名——`action list` 列词表，`action log` 列动作

### 0. 裁决

| 命令 | 作用 | 形态 |
| --- | --- | --- |
| `kiln action list [--json]` | 列出可用的 action 类型（词表、起点、终点） | 文本一行一个；`--json` 一份文档 |
| `kiln action log [--jsonl] [--limit N]` | 按时间列出动作记录（动作实例） | 文本一行一条；`--jsonl` NDJSON；`--limit` |
| `kiln action <future>` | `action` 组后续可扩展 | 每个新子命令单独定义边界，不在本次预埋 |

一句话：**`action list` 是 schema，`action log` 是 events；两者都在只读的 `action` 组下。**

### 1. 为什么是这两个名字

- `action list` 最自然地被读成“有哪些 action 可用”；把它给词表，消除了上一版“`action list` 到底是词表还是实例”的歧义。
- `action log` 与 `action list` 成组：`list` 列类型，`log` 列实例；用户不用在 `timeline` 与 `action list` 两个命名空间之间跳。
- `log` 有作用域限定（`action`），不是顶层 `kiln log`；`git log`、`docker container logs`、`kubectl logs` 都是只读的“看日志”，不是“写日志”。
- `log` 的审计联想必须写死边界：**它是当前提交的只读视图，不是改写史，也不是审计**；溯源仍回 jj `op log` / git `reflog`。
- `timeline` 继续作为**概念名**（动作时间线）留在文档里，但不再是命令名；这样既保留“时间线”的表达，又让命令面稳定。

help 文案必须把两句话放在第一行：

```text
kiln action list   列出可用的 action 类型（词表）；不是动作历史
kiln action log    按时间显示动作记录（只读）；不是审计 / 改写史
```

### 2. `kiln action list` 规格

```text
kiln action list [--json]
```

- 输出生效的动作词表：`.kilnconfig.jsonc` 的 `actions`，没有则内置默认；
- 文本：一个 action 一行，标注 `start` / `terminal`；无表头、无 legend，方便 `| wc -l`；
- `--json`：
  ```json
  {
    "actions": {
      "names": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
      "start": "ask",
      "terminal": ["conclude", "apply"]
    },
    "source": "file",
    "config": "C:\\Users\\jacki\\kiln\\.kilnconfig.jsonc"
  }
  ```
  - `source`：`file` 或 `default`；`config` 仅在 `file` 时出现；
  - `actions` 与配置文件的 `actions` 组同形，便于对照；
- **不需要仓库**：仓外/未初始化用默认词表，`source=default`；不读历史、不开后端；
- 不做 `--jsonl` / `--limit` / 过滤；与 `do` / `commit` 共用同一个 `ActionSet`，列出的就是写侧接受的集合。

### 3. `kiln action log` 规格

```text
kiln action log [--jsonl] [--limit N]
```

- 全局动作流：只留同时带 `issue:` + `action:` 的提交；author-date 倒序；
- 默认文本：一行一条，含日期 / issue / action / 短 hash / subject；不做分组、ordinal、relations、stat；
- `--jsonl`：一行一个动作对象，字段沿用 `timeline-stream` 的 `issue / action / commit / date / title / domain / tags`；
- `--limit N`：最多 N 条动作后正常退出；N ≥ 1；非法 exit 2；
- 不需要 `--all`（天生全局）；不提供 `--json`（一份数组会和 `--jsonl` 争视图）；不做过滤；
- 不支持 jj：显式非 0 + stderr「暂不支持 jj」，不静默空输出；
- 空仓/无动作：stdout 为空、exit 0；broken pipe 及时停扫描并结束 git 子进程；
- **不按当前词表过滤历史**：旧/非法 action 仍会出现，合法性归 `check`。`action list` 是 schema，`action log` 是历史事实，两者不能互相修剪。

文本是最小可读列表，不是富 timeline 视图：不做高亮、分组、交互；这些东西若要做，另开 issue。

### 4. 与现有命令的边界

| 命令 | 单位 | 边界 |
| --- | --- | --- |
| `list` | issue | issue 索引；`--action` 是“出现过某类动作的 issue”，不是动作列表 |
| `list --action=X` | issue | 过滤 issue，不是 `action list` 的替代 |
| `path <issue>` | 单 issue 的 path | issue 内的动作序列 + 关系/ordinal |
| `show <issue> n` | 单个动作 | 动作详情；不做全局寻址 |
| `action list` | 词表 | 当前可用的 action 类型；不读历史 |
| `action log` | 动作实例 | 全局历史；不按当前词表过滤 |

### 5. `action` 组的未来

- 本次只落 `list` / `log`；不预埋 `action show` / `action search` / `action filter`。
- 以后要加，仍在 `action` 组里加；每个新子命令必须先回答“与 `show <issue> n`、`list --search`、`path` 的边界是什么”，避免同一个问题有两个入口。
- 组内子命令保持只读；写操作继续走顶层 `do` / `commit`。
- `kiln action` 不带子命令时打印组用法（与 `kiln worktree` 一致）；具体文案 apply 定。

### 6. 实现切分

- `action list` 小、只读配置，先落；`action log` 需要流式后端与 NDJSON 契约，后落。
- 二者可以各开一个实现 issue，也可以在同一个实现 issue 里分两个 `apply`；本 issue 只负责命名与形态，不再改代码。
- `apply` 前仍需定死：`action list` 文本标记格式、`source`/`config` 省略规则；`action log` 文本列宽/日期格式、NDJSON 字段的 `title`/`domain` 语义、`--limit`/broken pipe/空仓、jj 错误文案、commit-graph 分档如何写进验收。

### 7. 本结论的验收

- 文档明确 `action list` = 词表、`action log` = 动作流，且说明 `log` 只读、非审计；
- 明确 `action` 组可扩展，但本次不预埋其它子命令；
- 现有命令零改动，`go test ./...` 不受影响。
