# issue: ci-portability — 发布流水线在 Linux 上挂：时区、分隔符、git 身份、worktree ENOTDIR

## 背景

`v0.4.0` tag 推送后，release workflow 的 `go test` 在 ubuntu 上失败（run 40940538450，
8 秒即失败），构建与 publish 被跳过，release 没有创建。日志（`job-logs.txt`）是 4 类问题，
全部是「只在 Windows 本地跑过」的测试可移植性缺陷，不是产品缺陷：

1. **时区**：golden 的时间列按 `time.Local` 渲染；夹具在 UTC+8 机器录制（`17:00`），CI runner
   是 UTC（`09:00`）。`TestMain` 只设了 `TZ=UTC`，但 testing 框架启动时已经读过时钟，
   `time.Local` 早已解析，改环境变量无效。
2. **路径分隔符**：`config_path` 的 golden 记录 `<ROOT>\.kilnconfig.jsonc`（Windows），
   Linux 输出 `<ROOT>/.kilnconfig.jsonc`；`normalizeGolden` 只替换 `<ROOT>`，没统一分隔符。
3. **git 身份**：`TestSnapshotFollowsHead` 的 `git commit --amend` 没带
   `GIT_AUTHOR_*` / `GIT_COMMITTER_*`；runner 没有全局 `user.name/email` →
   `Committer identity unknown`。
4. **worktree ENOTDIR**：`TestCacheIsSharedAcrossWorktrees` 用 `os.IsNotExist` 判断
   「worktree 的 `.git` 下没有 `kiln`」；linked worktree 的 `.git` 是文件，Linux 上
   `stat` 返回 ENOTDIR（不是 ENOENT），于是被判定为「存在」；Windows 返回 ENOENT，本地通过。

根因：release workflow 只在 tag 上跑测试，master 的日常推送没有 CI，所以这些缺陷从 v0.3.0
之后一路没被发现。

## 要做的

1. `TestMain`：`time.Local = time.UTC`（不只设 `TZ`）；
2. `normalizeGolden`：把 `<ROOT>\` 统一成 `<ROOT>/`；
3. `TestSnapshotFollowsHead`：amend 带上测试身份；
4. `TestCacheIsSharedAcrossWorktrees`：`stat` 任何错误都算「没写」；
5. 重录 golden（UTC）；
6. 新增 `.github/workflows/ci.yml`：master push / PR 跑 `go test ./...`，让同类问题在合并前
   暴露；release 仍只在 tag 上跑。

## 非目标

- 不改产品代码与输出；
- 不改 release workflow 的构建与发布逻辑。

## 验收

- 本地用「无全局 git 身份 + UTC」复跑全绿（`go test -count=1 ./...`）；
- golden 时间列为 UTC，`config_path` 不再有平台分隔符差异；
- `ci.yml` 在 push / PR 上跑测试，`release.yml` 不变。
