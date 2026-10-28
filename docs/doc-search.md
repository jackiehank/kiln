# issue: doc-search — 在 issue 文档正文里检索（暂不实现，先记账）

- **domain**：cli
- **关联**：relates-to `action-log`（本轮 900 行文档找不到刚写的“下游接入”一节，是这条的直接触发）；relates-to `search-result-contract`（issue 级搜索的既有契约）；relates-to `cli-necessity`（判断这条是不是只是 ripgrep 的包装）；relates-to `machine-readable-output`（若出 JSON，形状要跟既有约定一致）

## 背景

`action-log` 长到 ~900 行后，想找“下游接入”那一节，kiln 自己能提供的只有间接路径：

```text
$ kiln list --search 下游接入      # 命中 action-log，但因为它正好在动作标题里
$ kiln path action-log             # 13 个动作，看到序号
$ kiln show action-log 13          # 展开那一次提交的 diff，正是追加的那一节
```

这条链能用，但**只有关键词落在 issue id / 当前标题 / 动作 subject 上才成立**。埋在段落里的词，kiln 读不到；现在只能 `rg -n '关键词' docs/`，还得先猜文件名。

本轮实测的现状：

| 入口 | 搜什么 | 是否覆盖正文 |
| --- | --- | --- |
| `kiln list --search` | issue id / 当前标题 / 动作 subject（空白分词、词间 AND、字面量、大小写不敏感） | 否 |
| `kiln show <issue> <n\|action>` | 按序号或动作名展开，不按关键词 | 否（但能精确读到“哪一次改动”） |
| `kiln action log` | 全局动作流，不搜内容 | 否 |
| `rg` | 文件系统全文 | 是，但不知道 issue 与动作的对应关系 |

没有 `kiln search`；`action show` / `action search` 在 `action-view` 里被明确列为非目标。

## 要回答的问题

1. **触发条件**：这条当前只是记账。什么信号算“需求频繁”？——例如一周内 ≥N 次、或插件侧提出“按关键词定位到 issue 段落”的具体用法；达不到就继续用 `rg`。
2. **形态**：新命令（`kiln search`）／扩 `list --search` 到正文／给 `show` 或 `path` 加 `--grep`／都不做。按 `cli-necessity`，如果只是“`rg docs/` 的包装”，它不该是命令；kiln 独有的价值是**知道文件属于哪个 issue、哪一次动作**，那才是论点。
3. **搜哪些文件**：issue 文档的路径现在不在模型里（`path --json` 没有 docs 路径，只有动作与 hash）。是扫 `docs/` 全目录，还是从提交里取被改过的文件？后者要读 diff，成本完全不同。
4. **输出形状**：`文件:行:片段` 的文本，还是 `{issue, action, file, line, text}` 的 JSON？多命中时如何排序（issue、动作序号、行号）？
5. **匹配语义**：字面量（像 `list --search`）还是正则？多词 AND？大小写？是否与 `list --search` 保持同一套规则，避免“两个搜索两种行为”。
6. **性能**：900 行 × 68 个 issue 的 `rg` 是毫秒级；如果改成“读每个动作的 diff”就会退一个量级。要不要限定“只搜最终文件”，不搜历史版本？
7. **历史版本**：搜“曾经存在过但后来删掉”的内容吗？`git log -S` 能做，但那已经是另一个命令（历史考古），不是本 issue。

## 非目标

- 不做全文索引 / 倒排 / 缓存；
- 不做交互式浏览（分页、跳转、高亮）；
- 不改 `list` / `show` / `path` 的现有行为（真要加 `--grep` 时再单独评审）；
- 不做跨仓库搜索。

## 记录

- 2026-10-28：由 `action-log` 的文档检索痛点触发，先记 issue，暂不 hypothesize；等使用频率上来再讨论形态。
