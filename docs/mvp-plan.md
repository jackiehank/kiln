# kiln MVP 方案

> 版本：v0.1（MVP）｜语言：Go 1.27｜形态：单文件 CLI（`kiln`）

## 1. 背景与定位

kiln 不发明新的存储格式，也不接管文件创建。它把自己定位成 **“Git 提交信息的规范器 + 研究路径的重建器”**。

一句话：**kiln 是过滤器，不是注册器。** 它只做两件事：

1. **写**：把一次研究动作（提问 / 假设 / 采集 / 验证 / 结论 / 落地）编码成一段**携带语义 trailer 的规范提交信息**。
2. **读**：从 Git 历史里按约定**过滤**出 kiln 动作，进而重建某个 issue 的研究路径、关系网与完整性。

所有状态都住在 Git 提交信息里，kiln 自身**不落任何数据库 / 索引 / 注册表**。这样做的收益：

- 零新增存储、零服务依赖，`git clone` 到哪，研究历史就跟到哪；
- 天然对 Agent 友好——底层是 Agent 最熟悉的 `git`，kiln 只在顶层提供结构化语义；
- 撤销、回滚、跨分支复用全部复用 Git 既有能力。

### 设计原则（从讨论中提炼）

| 原则 | 含义 |
| --- | --- |
| 无模式（no modes） | 没有“纯变更模式 / 变更+记录模式 / 纯记录模式”之分，一切靠 trailer 约定 |
| 不建文件 | `q1.md`、`h1.md` 等记录文件由人或 Agent 自建，kiln 不代劳 |
| 不区分变更类型 | 新增还是修改对 kiln 无意义，kiln 不看 diff |
| 宽容输入，严格输出 | 解析时大小写不敏感、容忍冒号前后空格；生成时输出规范格式 |
| Agent-native | 结构化输出、语义清晰、单一职责、错误可操作、无隐式状态 |
| LLM 先验优先 | 只组合模型最熟的原语（git、JSON、REST-ish 文本），不为优雅引入长尾 DSL |

## 2. 核心约定

### 2.1 识别规则（唯一的“签名”）

> **一个提交是 kiln 动作，当且仅当它同时包含 `issue:` 与 `action:` 两个 trailer。**

- 只有其中一个（如普通提交里恰好写了 `issue: 性能瓶颈`）→ 忽略。
- 两个同时出现 → 纳入，几乎不可能被无意写出，误判率极低。
- 误判的后果仅仅是路径里多一条记录，不是错误；`kiln check` 可辅助发现漏写。

### 2.2 Trailer 规范

**必需（同时出现才构成 kiln 动作）**

| Trailer | 含义 | 示例 |
| --- | --- | --- |
| `issue:` | issue ID | `session-expiry` |
| `action:` | 动作类型 | `ask` / `hypothesize` / `collect` / `test` / `conclude` / `apply` |

**可选（描述）**

| Trailer | 含义 | 示例 |
| --- | --- | --- |
| `title:` | 人类可读标题 | `X 并发可能出错` |
| `refs:` | 引用的对象 | `q1, h1` |
| `target:` | 涉及的文件 | `src/x.go` |
| `scope:` | 描述性范围 | `函数 X 的并发锁` |
| `domain:` | 领域分类 | `auth` |
| `tags:` | 标签（列表） | `[security, performance]` |

**可选（关系）**

| Trailer | 含义 | 方向 |
| --- | --- | --- |
| `derived-from:` | 这个 issue 从哪来 | 子 → 父 |
| `spawns:` | 这个 issue 引出了什么 | 父 → 子 |
| `relates-to:` | 相关但不派生 | 双向 |
| `blocks:` | 阻塞了什么 | 阻塞者 → 被阻塞者 |
| `supersedes:` | 取代了什么 | 新 → 旧 |

### 2.3 action 词表（可配置）

默认词表：`ask` / `hypothesize` / `collect` / `test` / `conclude` / `apply`
默认起点：`ask`；默认终点：`conclude`、`apply`（`apply` 表示结论已落地到代码）。

**顺序不是约束**：`ask` 只负责“开启一个 issue”，其后的动作是一个**无序集合**——任意组合、任意次序都合法，不存在“必须先 hypothesize 再 test”之类的流水线要求。正常情况一个 issue 只有一个 `ask`；同一 issue ID 下再出现 `ask`，实际属于另一个 issue（该识别留待后续迭代）。

词表由当前目录的 `.kiln.json` 覆盖（不存在则用上面的默认值）：

```json
{
  "actions": ["ask", "hypothesize", "collect", "test", "conclude", "apply"],
  "start": "ask",
  "terminal": ["conclude", "apply"]
}
```

- `actions`：合法词表（大小写不敏感，入库时归一为小写）
- `start`：开启 issue 的动作，缺省取 `actions` 首项
- `terminal`：关闭 issue 的动作集合，缺省取 `actions` 末项
- `start` / `terminal` 不在 `actions` 中，或词表为空/有重名 → 启动时报错

最小配置只需列出词表，例如 `{"actions": ["ask", "probe", "done"]}` → `start=ask`、`terminal=done`。

### 2.4 issue ID 命名约定

- 格式：单段 **kebab-case slug**，如 `session-expiry`、`action-config`
- 规则：小写；只允许字母、数字、`-`；长度 ≤ 50；不允许 `/`
- 唯一性范围：单个仓库内
- 唯一性**不靠注册表**，靠命名约定 + `kiln check` 检测 + 人工确认

> 早期设计用过 `<namespace>/<slug>`（namespace = 项目名或创建者名）来降低重名概率，并给跨仓库引用留了 `repo-name/...` 前缀。但单仓库、单人使用时 namespace 恒等于项目名、零信息量，于是**去掉**：ID 收敛为单段 slug。去重与跨仓库引用怎么做，已作为 issue `status-by-domain` 派生的独立问题追踪。

### 2.5 规范提交信息样例

```
发现 X 并发可能出错

issue: session-expiry
action: ask
title: X 并发可能出错
derived-from: deep-dive
```

标题行与 trailers 之间**留一个空行**：git 会把没有空行分隔的整段当作同一个 subject，导致 `git log --oneline` / PR 标题把 trailer 一起挤进去。

### 2.6 术语：为什么用 `issue`

被追踪单元最初叫 `problem`（“缺陷”含义偏重），而它的本质是“以 `ask` 开启、经若干动作、以终点动作收尾”的一条**探究线**，并不必然是一个缺陷。经评估后**改用 `issue`**（trailer 键 `issue:`）：

| 候选 | 结论 |
| --- | --- |
| `issue` | ✅ 采用。GitHub issues 是最主流的追踪术语，LLM 先验最强，覆盖 question / bug / task |
| `problem` | 弃用。工程直觉强，但英文隐含“有毛病”，与 ask 驱动的探究语义有偏差 |
| `question` | 弃用。与起点动作 `ask` 呼应，但终点常是 `apply`（落地代码）时语义别扭 |
| `topic` / `inquiry` | 弃用。中立贴“研究路径”，但前者略泛、后者 LLM 先验弱 |

中文“问题”同时覆盖 issue / question / problem，因此中文文案沿用“问题”不影响理解，**英文键名统一为 `issue`**。

> 迁移记录：本仓库早期 5 条提交曾用 `problem:`，已通过改写历史统一为 `issue:`。

## 3. 命令面（CLI）

| 命令 | 作用 | MVP |
| --- | --- | --- |
| `kiln do <issue> <action>` | 生成规范提交信息（**只打印，不写文件**） | ✅ |
| `kiln commit <issue> <action>` | 用规范信息直接提交 | ✅ |
| `kiln path <issue>` | 重建该 issue 的研究路径 | ✅ |
| `kiln status` | 全库概览 | ✅ |
| `kiln check` | 语义完整性校验 | ✅ |
| `kiln list [--domain=] [--tag=] [--action=]` | 按条件过滤 issue | ✅ |
| `kiln graph [--format=ascii\|dot\|mermaid]` | issue 关系图 | 仅 `ascii` |
| `kiln rename <old> <new>` | 重命名 issue 并改写相关 trailer | ❌（后续） |

### 输出示例

```
$ kiln do session-expiry ask --title "X 并发可能出错"
发现 X 并发可能出错

issue: session-expiry
action: ask
title: X 并发可能出错
```

```
$ kiln path session-expiry
session-expiry  进行中（当前 test）
  1. ask          a1b2c3d  发现 X 并发可能出错
  2. hypothesize  e4f5g6h  假设锁竞争导致
  3. test         i7j8k9l  压测复现
  关系: derived-from deep-dive
```

```
$ kiln check
✓ session-expiry   完整（ask, hypothesize, test, conclude）
⚠ deep-dive        缺少终点动作（当前 test）
✗ broken-ref            父 issue 不存在：derived-from 指向 missing-parent
```

## 4. 数据模型（Go）

```go
// 一次被识别为 kiln 动作的提交
type Action struct {
    Hash     string
    Subject  string
    Date     time.Time
    Issue    string
    Kind     string   // ask|hypothesize|collect|test|conclude|apply
    Title    string
    Refs     []string
    Target   string
    Scope    string
    Domain   string
    Tags     []string
    DerivedFrom []string
    Spawns      []string
    RelatesTo   []string
    Blocks      []string
    Supersedes  []string
}

// 由若干 Action 聚合出的 issue
type Issue struct {
    ID      string
    Actions []Action // 按时间升序
    Domain  string
    Tags    []string
}

// 全库聚合结果（引擎的输出，CLI 只消费它）
type Repo struct {
    Issues map[string]*Issue
}
```

## 5. 架构与模块划分

```
kiln/
├── cmd/kiln/main.go         # 进程入口
├── internal/
│   ├── model/               # 领域类型（Action / Issue / Repo）+ ActionSet 词表
│   ├── config/              # 读取 .kiln.json → ActionSet（缺省用内置默认）
│   ├── trailer/             # 提交信息 → trailers 的解析（纯函数，可单测）
│   ├── git/                 # Git 适配层：log / commit（接口 + exec 实现）
│   ├── engine/              # 聚合、path / check / status / list / graph
│   └── cli/                 # 子命令分发 + 参数解析 + 文本渲染
├── docs/mvp-plan.md
└── go.mod
```

分层约束：`cli → engine → (git, model)`，`trailer` 被 `git`/`engine` 复用。**引擎不直接调 exec**，只看 `git.Runner` 接口产出的提交列表——保证核心逻辑可用假数据单测。

## 6. MVP 范围 / 非目标

**做**：识别规则、trailer 解析、可配置 action 词表（`.kiln.json`）、`do`、`commit`、`path`、`status`、`check`（规则 1–4）、`list`、`graph --format=ascii`。

**不做（MVP 明确排除）**：文件创建、变更类型区分、模式切换、中央注册表、签名校验、`rename` 自动改写、`check` 的规则 5–7（阻塞状态 / 循环依赖 / 对象引用）、`--format=json`、MCP server。

## 7. `check` 校验规则

核心原则：**每个 issue 必须由起点动作开启、由终点动作收尾。**（中间动作不设顺序约束）

**MVP 必须实现**

1. 存在性：每个 issue 至少含一个 `start` 动作（默认 `ask`）
2. 终点：路径的最后一个动作必须是 `terminal` 动作（默认 `conclude` / `apply`）
   （不校验动作顺序；起点之后的动作是无序组合）
3. 父存在：声明 `derived-from` 时，父 issue 必须存在
4. 子存在：声明 `spawns` 时，子 issue 必须存在

**后续迭代**：阻塞已关闭 issue、`derived-from` 环检测。

输出分级：`✓` 完整 / `⚠` 警告 / `✗` 错误。约定：CI 中任何 `✗` 阻断合并，`⚠` 仅提示。

## 8. 里程碑与验收

| 里程碑 | 交付 | 验收 |
| --- | --- | --- |
| M1 解析内核 | `trailer` + `model` | 单测：识别规则、大小写/空格容错、tags 列表全部通过 |
| M2 引擎 | `git` 接口 + `engine`（path/check/status/list） | 用假提交集单测：路径重建、闭环校验正确 |
| M3 CLI | `do`/`commit`/`path`/`status`/`check`/`list`/`graph` | 在真实仓库跑通，输出符合示例 |
| M4 收尾 | `go vet`、`go test ./...` 全绿 | CI 可加 `go test` |

## 9. Agent 集成

让 Agent 知道约定的三种方式（任选，建议前两种）：

1. **Skill / 系统提示**：在 Agent 的 Skill 里写明“做研究动作时用 `git commit -m "$(kiln do <issue> <action>)"`”。
2. **`kiln do` 默认输出**：直接产出完整提交信息，Agent 复制即可，无需记忆格式。
3. **Git hook（可选，非 MVP）**：检测 missing trailer 时提示。

## 10. 后续迭代（MVP 之后）

- `kiln rename`：重命名 issue 并改写全部相关 trailer
- issue ID 唯一性：去掉 namespace 后靠什么避免重名——命名约定 + `kiln check` 检测是否够用，是否需要跨仓库引用前缀
- **kiln CLI 的存在必要性**：agent 手握 `git` + 约定文档时，哪些命令是"git 做不到的"（`check` 不变量 / `status`·`graph` 综合 / `path` 重建）、哪些只是方便包装——见 [docs/cli-necessity.md](cli-necessity.md)
- 动作「证据」可被改写 / 删除（jj 的流体历史）与「记录」取向的张力：**阻塞 cli-necessity**——见 [docs/evidence-fluidity.md](evidence-fluidity.md)
- 关系渲染不一致：`graph` / `status` 未渲染 `blocks` / `supersedes`（而 `path` 有）——见 [docs/relation-rendering.md](relation-rendering.md)
- 动作的**作者归属**：**已定"不做"**——现状 1 人 + 1 agent 共用同一 git 身份，且无消费场景；重启触发条件与理由见 [docs/author-attribution.md](author-attribution.md)
- 动作粒度：实现一个 issue 时，中间动作（`hypothesize` / `test` / …）要不要分阶段提交——见 [docs/action-granularity.md](action-granularity.md)
- 关系类型落地：新增 `part-of`（分解）；**取消反向名**（词表共 5 个名字），方向由**箭头**渲染——规格见 [docs/relation-vocabulary.md](relation-vocabulary.md) 的结论
- 关系词表规格：5 个名字、方向渲染、命名原则——见 [docs/relation-vocabulary.md](relation-vocabulary.md) 的结论
- issue「同源」的表达：两个 issue 共用**非 issue 的来源**（故障 / 论文 / 会议）时，用 `relates-to` 兜底还是新增对称关系 / 外部来源引用——见 [docs/common-origin.md](common-origin.md)
- 变更展示**已落地**：`path --stat` / `--shortstat` + `show <issue> [n]`（两层，对齐 git 的 log/show、委托 git 渲染）；`path --diff` 否决——见 [docs/action-diff.md](action-diff.md)
- 用 diff 反查 `target:` 声明是否属实：`check` 新增"声明 vs 现实"的一致性校验（候选 D，源于 `action-diff`）——见 [docs/target-honesty.md](target-honesty.md)
- 同一 issue ID 下出现第二个 `ask` 时的识别（视为新 issue 的分叉）
- `kiln graph --format=dot|mermaid`：接入外部渲染
- `kiln check --format=json`：CI 集成与 PR 评论
- 关系复杂度提升：`derived-from` 环检测、`blocks` 状态一致性
- 封装为 MCP server：把结构化工具暴露给 Agent，彻底避免其拼 CLI

## 11. `kiln status`：按 domain 分组的目录视图

`status` 以 `domain` 为“目录”：同一 domain 的 issue 成块相邻，跨域派生的 issue 以树形嵌在父 issue 下。

```
问题总数: 5（完成 4 / 进行中 1）｜domain 2 个
kiln 动作数: 9

cli/help-alignment     ✓ apply
cli/status-by-domain     ask

design/action-config   ✓ apply
design/commit-format   ✓ apply
design/problem-naming  ✓ apply
```

规则：

- 每行写作 `<domain>/<issue-id>`；**不设目录标题行**，块之间用空行分隔——域名由每行的前缀携带，所以跨域嵌套行的域名依然一眼可见。
- 块按 domain 名升序；块内按 issue ID 升序；未声明 domain 的 issue 归入 `(未分类)` 块，固定最后（这些行不带前缀，只写 ID）。
- 完成标记 `✓` 放在 ID 之后，进行中留空。
- 列宽取全部行的**显示宽度**最大值，跨块统一对齐。

### 11.1 跨域派生（嵌套）

若父 issue 的 `spawns:` 或子 issue 的 `derived-from:` 指向的 issue 处在**不同 domain**，则子 issue **移动**到父 issue 之下，以 `├──`/`└──` 呈现，**不在自己的 domain 块里重复**：

```
design/status-by-domain     ask
└── cli/help-alignment    ✓ apply
```

- 只有跨域关系才嵌套；同域关系保持平铺（关系视图交给 `kiln graph`）。
- 多个父声明同一个子时，按 ID 序取第一个；会成环的候选被跳过。

### 11.2 落点

- `engine.GroupByDomain(repo) []DomainGroup`：返回已排序、已嵌套的纯数据；单测覆盖分组/排序/`(未分类)` 兜底/跨域移动/同域平铺。
- `cli`：`statusLines` + `renderStatus` 负责渲染；`list`/`graph`/`check` 不受影响。

### 11.3 非目标

- 不拆嵌套 domain（`a/b` 是一个不透明键）；不改 `list`/`graph`；不新增 `--flat` 开关；不新增 check 规则。
- 兼容性：`status` 输出是**破坏性**变更（脚本可能依赖旧平铺格式），MVP 阶段无外部消费者。

