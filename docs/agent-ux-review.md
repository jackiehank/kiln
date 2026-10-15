# issue: agent-ux-review — agent 对 kiln 的评价与改进建议

- **domain**：cli
- **关联**：relates-to `refs-flag-splitting` / `relation-rendering`

## 背景

一名 agent 在**一个完整会话**里用 kiln 做完了 6 条 issue（`qualified-issue-id`、`issue-uniqueness`、`status` 视图改造等），期间高频使用了 `path` / `show` / `status` / `check` / `list` / `do` / `commit` / `-h`。用户要求它做一次**使用回访**：哪些高效、哪些反直觉、哪些试了多次才成。

本 collect 就是那次回访的一手记录。**它同时是证据**：每条都带可复现的命令与观察，供后续 `hypothesize` 挑可落地项，而不是一篇印象派感想。

**覆盖面的诚实说明**：`graph` 只用过一次；`worktree new/list/remove` 因本仓 `worktree: off` **没有真实使用**（只读代码与测试）；`--json` 用在 `status` / `list` / `check` 三个命令上。所以下面的意见对这三块是"空白"，不是"满意"。

## collect：agent 视角的使用回访

> 来源类型：**本仓的一手使用记录**（命令 + 输出 + 本会话踩坑），非联网调研。凡"已修"指本会话内已落地并有提交；"仍在"指访谈当时仍然成立。

### 1. 高效且符合直觉的（8 条）

1. **`-h` 一屏就能开工。** 命令名是动词、语义直白；读完 `-h` 不必翻 README 就能发出第一条命令——学习成本属于最低一档。
2. **`kiln do` 只打印、不落库，是全工具最有价值的单点设计。** 每次 `commit` 前都先用 `do` 零副作用地试 flags：
   `kiln do qualified-issue-id ask --title "…" --domain cli --relates-to issue-uniqueness`
   把"生成"与"落库"分开，对 agent 尤其划算。
3. **`kiln path <issue>` 是主力命令。** 一次给到「序号 + 动作名 + hash + 标题 + 关系」，一次会话里用了几十次。
4. **`--stat` / `--shortstat` 委托 git 渲染**，输出与 `git show --stat` 同形——工具没有自创展示语言，学习成本为零。
5. **`check` 的输出形状好。** `✓ action-diff 完整（ask, collect, hypothesize, test, conclude）`：逐 issue 报动作序列，审计友好；warn 不影响退出码、`--json` 的 `ok` 与退出码同义，语义干净。
6. **状态全在提交信息里 → 没被工具锁住。** 全程用 `jj st`（干净）+ `check`（全绿）当"这轮做完了"的双信号，随时可 `jj log` / `git show` 交叉验证；没有只在自家 TUI 里看得见的黑盒。
7. **拒绝语给可复制配方。** worktree 那套提示（"开工前先 `kiln worktree new <issue>`"、cd 到该区）是全场最好的错误处理；后来写新拒绝语时直接照抄了这种风格（`提示: 要继续它，用 kiln commit <id> <action>`）。
8. **文档约定可推断。** 一 issue 一 doc、`背景 / 要回答的问题 / collect / hypothesize / test / 结论` 分节、末尾"来源"一行——看 3 个文件就推出来了，之后几十次写入都没犹豫过放哪。

### 2. 反直觉 / 需要试探或读源码的（10 条）

| # | 现象 | 证据 | 状态 |
| --- | --- | --- | --- |
| 1 | `status` 打 `<domain>/<id>`，但输入只吃裸 id | `kiln path design/worktree-mode` → 「未找到问题」（会话第二条命令就踩） | **已修**（`qualified-issue-id`：块头显示 domain、行内裸 id） |
| 2 | **`show <issue> <n>` 的 `n` 是位置序号，随动作增加而漂移，且选错不报错** | 想取 test，`kiln show qualified-issue-id 4` 返回的是第二条 `hypothesize`；只有 header `4/6 hypothesize` 提示了错误，而注意力在下面的 diffstat 上（本会话踩了 **2 次**） | 仍在 |
| 3 | **`--target` 是标量，`--refs` / `--tag` 是列表**，但 `-h` 把它们排在同一行 | 给 `--target` 传 9 个逗号分隔的文件名，落成**一个字符串**（`writeScalar` + `f.get("target")`）；而 `--refs a,b` 会拆成两项。读了源码才确定 | 仍在（与 `refs-flag-splitting` 同族） |
| 4 | **关系是单向可见的**：`relates-to` 文档写"双向"，渲染却只从声明侧出 | `kiln path qualified-issue-id` → `关系: relates-to issue-uniqueness, …`；`kiln path issue-uniqueness` → `关系: derived-from status-by-domain`（**完全看不到有人 relate 它**）→ 反查"谁和我相关"只能靠 `graph` 或人工搜 | 仍在 |
| 5 | **`--domain` 看起来是"issue 的属性"，实际是"最后写入者胜的可选 trailer"** | 读 `engine.go` 才知道：后续任一动作都能改写它，且可以没有。而它同时影响 `status` 的显示与 `list --domain=` 的筛选 | 语义没变（本轮已写进 `mvp-plan` §2.4） |
| 6 | **`commit` 没有确认行** | 提交成功只把消息原样打印一遍，没有 `已提交 <hash>`；为确认"到底提没提"额外跑了 `path` + `jj log` 数次。对比 `worktree new` 会打印 `worktree: <path>`——同一工具两种确认风格 | 仍在 |
| 7 | **`-h` 不提「提交范围 / 暂存」** | 最危险的语义（jj 下 `commit` 固定**整个工作副本**；git 下必须先 `git add`）只在 README 里。agent 的默认动作是先读 `-h`，这次恰好先读了 README 才没出事 | 仍在 |
| 8 | 小不对称：flag 是 `--tag`（单数）、trailer 是 `tags:`（复数）；历史里还有 `object:`（现役 `do` 不产出） | `object` 只从一条旧提交 + "object 已退役"的用法报错里知道——**报错处理得算好的那类** | 仍在（措辞/兼容性小事） |
| 9 | 动作词表**无序**，但 `path` 的 `1..N` 编号像在描述时间线 | 文档两处明说"顺序不是约束"，使用者仍会把它读成"流程第 3 步"。编号作为索引有用，同时暗示了一条不存在的流水线 | 设计取舍，供参考 |
| 10 | 过时措辞把人引向不存在的功能 | `警告: issue ID "…" 不符合 <namespace>/<slug> 约定`，而 `-h`/README 里 namespace 早已删掉 | **已修**（本会话顺手改成单段 kebab-case slug） |

### 3. 失败 / 需要多次尝试的清单

| 命令 | 结果 | 次数 |
| --- | --- | --- |
| `kiln path design/worktree-mode` | 未找到问题（假设了 namespace）→ 改裸 id 成功 | 2 |
| `kiln show worktree-mode 2 --stat` | 动作序号超出范围（猜 2 个动作，实际 10 个） | 2 |
| `kiln show qualified-issue-id 4 --stat` | **不报错**，静默给了另一条动作 | 1（靠 header 发现） |
| 查"有没有重名" | `check` 不查这个 → 自己拼 `list --json \| grep id \| uniq -d` | 1 |

### 4. 至今不确定的

- **`target:` 该装"一个文件"还是"一串文件"**：传了 9 个，落成 1 个字符串，**没有任何地方说这是错的**（`check` 不校验、渲染看不出）。将来 `target-honesty`（用 diff 反查 `target:` 声明是否属实）会直接撞上这个——它得先决定怎么切分。
- **`status --json` 的 `domains` 在被搬空的组上的历史语义**：本轮修掉了，但是**读测试才发现**曾经不直观。

### 5. 如果只改三件事（按高频用户收益排序）

1. **`commit` 加一行确认**：`已提交 830a0cd · test · qualified-issue-id`。零成本，直接消掉最多的"再跑一条命令确认"。
2. **让 `show` 的错位可见或可避**：允许按**动作名**选（`kiln show <issue> test` 取最后一次该动作？），或至少把 `4/6` 提到与标题同一行。序号漂移是设计选择，"选错不报错"不是。
3. **`-h` 补一行「提交范围」**：jj＝固定整个工作副本、git＝需先 `add`、`--force` 是什么。这是唯一"错了会污染历史"的语义，不该只藏在 README。

> 收敛成一个主题：**flag 与输出的"形状"应当自证**——标量 vs 列表、可选 vs 必填、可变 vs 不可变、序号是否稳定。这四类信息里，目前只有"必填"在 `-h` 上写清楚了，其余都得读源码或试错。

> 来源：**本仓的一手使用记录**（agent 在 `qualified-issue-id` / `issue-uniqueness` / `status` 展示改造等 6 条 issue 上的完整会话：命令、输出、失败次数）；其中「已修」两项分别落在 `qualified-issue-id`（`status` 画法）与 `issue-uniqueness`（警告措辞与写侧归一化）的提交里。相关既有追踪：`refs-flag-splitting`（flag 形态，本条 issue 自身尚无 doc）、[relation-rendering](relation-rendering.md)（关系渲染不一致）、[target-honesty](target-honesty.md)（`target:` 声明与实际 diff）、[issue-list-scaling](issue-list-scaling.md)（全量输出与无索引）。
