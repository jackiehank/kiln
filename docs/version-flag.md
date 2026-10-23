# issue: version-flag — `kiln --version` 报出版本与构建时间

- **domain**：cli
- **关联**：relates-to `browser-shot`（同为访谈里"工具本身要可信"的一类）

## 背景

agent 访谈（2026-10-23）P6：本机 PATH 上的 `kiln` 是仓库根 `kiln.exe` 的符号链接，而该文件可能是一天前的旧构建——它对新 flag 的行为是 `警告: 未知 flag --json` 然后静默回退，看起来像"功能没实现"，排查花了一轮。现在没有任何一条命令能回答"我跑的到底是哪一版、什么时候构建的"。

`kiln` 没有版本号体系，也不打算为此引入发布流程；要的是**构建事实**：二进制路径、构建时间（可执行文件的 mtime 是最接近"这个文件何时生成"的事实）、go 版本与平台。

## 问题

1. 版本从哪来：`runtime/debug.ReadBuildInfo()` 的 `Main.Version`（`go build ./cmd/kiln` 下是空 / `(devel)`）；不引入 ldflags，不维护手写版本号。
2. 构建时间从哪来：可执行文件 mtime（`os.Stat(os.Executable())`），不用 commit 时间——它回答的是"这个文件新不新"。
3. 形态：`--version` / `-V` / `version` 三种写法是否都支持？决策：都支持、输出逐字节相同；`kiln version` 与 `kiln --version` 互为别名。
4. 位置：版本不依赖配置与仓库，`kiln --version` 在坏配置、非仓库目录下也要能跑。

## 验收

- **E1**：`kiln --version` / `kiln -V` / `kiln version` → exit 0，输出逐字节相同。
- **E2**：输出含 `kiln <version>` 行（`devel` 可接受）、`可执行文件: <绝对路径>`、`构建时间: <exe mtime>`、`go<ver> <os>/<arch>` 行。
- **E3**：不需要配置、不需要 git/jj 仓库；坏 `.kilnconfig.jsonc` 下仍 exit 0。
- **E4**：不写 stderr；不出现给 JSON 契约用的字段（这是人读命令）。
- **E5**：`go build ./...` / `go test ./...` 全过；`kiln -h` 有这一行。

## 非目标

- 不做自更新、不做 `kiln upgrade`；
- 不校验源码是否比二进制新（那是 CI / 构建脚本的事；版本命令只报事实）；
- 不把版本塞进 `status --json`。

## apply（2026-10-23）

- `internal/cli/cli.go`：`Run` 在命令分发里加 `version` / `--version` / `-V`，且**先于**配置错误检查返回——版本不读配置、不读仓库。新增 `printVersion(io.Writer)`：
  - `kiln <version>`：`runtime/debug.ReadBuildInfo().Main.Version`，空 / `(devel)` 时回退 `devel`；本仓 `go build` 得到的是 Go 的伪版本（脏树带 `+dirty`），这正是"这份二进制来自哪次构建"的事实；
  - `可执行文件: <os.Executable()>`；
  - `构建时间: <exe mtime>`（`os.Stat` 失败则省略该行，不编造时间）；
  - `<runtime.Version()> <GOOS>/<GOARCH>`。
- `usageCommands` 增加一行 `kiln --version`，说明三种写法同义。
- 测试：`TestVersionPrintsTheBuildFacts`（三种写法逐字节相同、含三行事实、不写 stderr）、`TestVersionNeedsNoConfig`（临时目录里放坏的 `.kilnconfig.jsonc`，仍 exit 0）。
- 复核：`go build` / `go vet` / `go test ./...` 全过；本机三种写法输出一致；坏配置下 `--version` 正常。

## 结论（已定）

`kiln --version` / `kiln -V` / `kiln version` 是同一个命令，输出：

```
kiln v0.0.0-20261023091450-476793954e64+dirty
可执行文件: C:\…\kiln.exe
构建时间: 2026-10-23 17:15:50
go1.27.0 windows/amd64
```

它回答的是"我跑的是哪一个二进制、它多久没重建了"，不是"源码是什么版本"：`构建时间` 取可执行文件 mtime，`version` 取 Go 构建信息——不维护手写版本号、不走 ldflags、不碰发布流程。坏配置、非仓库目录下都能用，因为它不读这两样。
