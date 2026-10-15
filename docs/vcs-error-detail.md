# issue: vcs-error-detail — 失败的报错丢了解释：只取 stderr，而 git 把说明写在 stdout

- **domain**：cli
- **关联**：relates-to `agent-ux-review`

## 背景

在 `agent-ux-review` 的沙盒里（临时 git 仓库、`worktree: off`），忘了 `git add` 就提交，得到的是：

```
$ kiln commit demo test --title "没暂存"
提交失败: git commit: exit status 1        ← 原因呢？
```

而 git 自己明明说了（把 stdout 留下、丢掉 stderr 即可看到它走的是 stdout）：

```
$ git commit -m x
On branch master
Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
```

**诊断**：[`internal/vcs/git/git.go`](../internal/vcs/git/git.go) 的 `runIn` 注释写着 "on failure the error carries git's **stderr** so the caller can surface a useful message"——意图是对的，但这一次 git 把说明写在 **stdout**，`stderr` 取空后退回 `err.Error()`，于是只剩 `exit status 1`。jj 侧的 `run` 是同一形状（只取 stderr）。

它和刚做完的「`-h` 补提交范围」是同一件事的两半：**规矩说了（git 需先 `git add`），但犯规时的报错不说原因。**

## 要回答的问题

1. 失败时到底该带哪一路输出——`stderr → stdout → err.Error()` 三级兜底？
2. 带**多少**：全文 / 首行非空 / 截断？一行错误里塞进 `git commit` 的完整建议会很吵。
3. jj 侧要不要对称处理（它多数写 stderr，但对称更省心、更少特例）？
4. 与既有规矩的关系：这不违反"提示只进 stderr"——它是**错误内容**而不是提示，仍然只在失败时出现在 stderr，只是内容更完整。

## 候选

| | 做法 | 代价 |
| --- | --- | --- |
| **A（倾向）** | 失败时 `stderr` 为空则退到 `stdout`，并只取**首行非空** | 最小、可控；首行通常就是 `On branch master` 这类上下文 |
| B | 三级兜底但带**全文** | 大段输出（git 的建议常有 4–6 行）会污染错误行 |
| C | 不动适配器，在 `write` 里按命令名特判 `git commit` 失败、给专门提示（"通常是没有暂存内容：先 `git add`"） | 脆（靠命令名分派）、且只覆盖 `commit` 一条路径 |

## 非目标

- 不改退出码语义（失败仍是 1）；不改成功路径的输出。
- 不把 git 的 stdout 当作**返回值**的一部分——只在**失败**时兜底成错误内容。
- 不做"错误分类 / 错误码"的体系化改造（那是另一件事）。

## 验证方式（待 `test`）

一条真仓测试：干净仓库里 `commit` 且无暂存 → 错误串里应当出现 git 的解释（如 `not staged`），而不只是 `exit status 1`。

## test：先按 A 的字面版跑了真沙盒——**首行被输出否决，改用末行**

> 用户要求：先按 A 方案（三级兜底 + 取**首行**非空）跑一次，看它会输出什么。

### 1. A 的字面版：输出是残废的

沙盒（`/tmp/kiln-sandbox`，改了 `demo.md` 但没 `git add`）：

```
$ kiln commit demo test --title "没暂存"
提交失败: git commit: On branch master        ← 给了上下文，没给原因
```

对照 git 自己的完整说明（它在 **stdout**）：

```
On branch master
Changes not staged for commit:
  (use "git add <file>..." to update what will be committed)
  ...
no changes added to commit (use "git add" and/or "git commit -a")
```

→ **git 的建议是“上下文在前、结论在后”**，首行恰好是最没用的一行。 这就是“先跑一次看输出”的价值：纯靠推理会把 A 当成对的。

### 2. 定稿规则（A′：只把“首行”改成“末行”）

- `stderr` 有内容 → 照旧用整段（常见路径；`fatal:` 一类错误通常就一行）；
- `stderr` 为空 → 退到 stdout 的**最后一个非空行**；
- 两者皆空 → `err.Error()`。

同场景输出：

```
$ kiln commit demo test --title "没暂存"
提交失败: git commit: no changes added to commit (use "git add" and/or "git commit -a")
```

一行里同时给了**原因**与**修法**。

### 3. 实现时发现的重复，并已收拢

写这个兜底时发现 [`jj`](../internal/vcs/jj/jj.go) **早就有**一个一模一样的 `lastLine`（给 `ShowStat` 取单行 stat 用）。于是没有新增两份，而是：

- 新增 [`vcs.LastNonEmptyLine`](../internal/vcs/vcs.go) 作为**唯一实现**，两个适配器都用它；
- jj 侧删掉本地 `lastLine`（`ShowStat` 改调共用版），并删掉它的 `TestLastLine`——覆盖范围已由 `vcs_test.go` 的新测试接手（同一场景 + 空输入）。

### 4. 验证

- 新增单测：`TestLastNonEmptyLineTakesTheConclusion`（vcs：取末行 + 空/空白输入）与 `TestActionReportsGitsReason`（真仓：干净仓 `commit` 无暂存 → 错误串含 `nothing to commit`，且**不含** `On branch`）。
- `gofmt -l` 空、`go vet ./...` 空、`go test ./...` **全绿**。
- 真机输出：上文两条（字面版 vs 末行版）。

### 5. 候选 B / C 为何不需要

末行已经能一行说清“原因 + 修法”，所以 **B（带全文）**的多行污染不需要付；**C（按命令名特判）**的脆性也不需要（兜底逻辑与命令无关）。

## 结论（已定）

1. **兜底顺序定稿**：`stderr` 有内容 → 用它（整段 trim）；为空 → 退到 stdout 的**最后一个非空行**；再空 → `err.Error()`。两个适配器共用 [`vcs.LastNonEmptyLine`](../internal/vcs/vcs.go)（唯一实现）。
2. **为何是末行而不是首行**：失败输出是**上下文在前、结论在后**的形状——实测首行给的是 `On branch master`，末行给的才是 `no changes added to commit (use "git add" and/or "git commit -a")`。规则跟着**输出的形状**走，而不是跟着“哪一路（stdout/stderr）”走。
3. **只取一行**：错误保持一行（B 的多行污染否决）；兜底与命令无关（C 的命令名特判否决）。
4. **不变量**：失败仍 exit 1；stdout 只在**失败**时被读作错误内容，成功路径的返回值不变。至此与 [`-h` 的「提交范围」](../internal/cli/cli.go) 合上——那个讲**规矩**，这条讲**犯规时的原因**。
5. **顺带收拢重复**：jj 里那个同名的 `lastLine`（`ShowStat` 用）已删，改用共用实现；它的单测覆盖搬到 `vcs` 层。

> 一句话：**报错要回答“为什么”，而工具把“为什么”写在了最后一行。**
