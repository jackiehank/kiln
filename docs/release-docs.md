# issue: release-docs — 发版后同步 README 里的版本引用

- **domain**：cli
- **关联**：relates-to `version-flag`（`--version` 的语义：release 产物显示 tag 名）；relates-to `doc-search`（同期只做记录、避免 README 继续堆历史）

## 背景

README 安装段与 `--version` 段各有一处版本示例，写的是上一次发布的 `v0.2.0`：

```text
README.md:45   也可以用 `@v0.2.0` 之类的版本写法固定版本。
README.md:59   release 产物显示 tag 名，如 `v0.2.0`。
README.en.md:43  Pin a version with `@v0.2.0` if you like.
README.en.md:57  release artifacts print the tag, e.g. `v0.2.0`.
```

`v0.3.0` 已经发布，这四处现已过期。它们不是“历史记录”，是**当前用法示例**，所以必须跟着最新 tag 走。

## 要做的

1. 四处示例改成 `v0.3.0`；
2. 明确一条维护规则：README 里只允许“示例版本”随最新 tag 更新，**不写 changelog、不列历史版本**——历史在 Releases 页与 `git tag` 里；
3. 其他文档里作为**历史用例**出现的 `v0.2.0` 不动（例如 `search-result-contract` 里那条真实 query 例子）。

## 非目标

- 不做 CHANGELOG 文件（release notes 由 workflow 从提交生成）；
- 不改安装方式、不动 release workflow；
- 不批量重写文档里的历史版本号。

## 记录

- 2026-10-28：v0.3.0 发布后同步。
