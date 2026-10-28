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
