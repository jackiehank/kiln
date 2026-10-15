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
