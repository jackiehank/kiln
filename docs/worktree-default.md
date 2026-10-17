# worktree-default

一句话：`worktree` 的默认值该是 `on` 还是 `off`？

- **domain**: cli
- **状态**: 进行中 → 见 `kiln path worktree-default`

## 开题（ask）

隔离（`worktree: on`）是**行为性护栏**：main 拒绝记录动作，必须 `kiln worktree new <issue>`
先开一个专用区。它解决的问题是真的——并行 agent 在同一个工作副本里互相污染，未提交
改动会被混进同一条提交，动作归属就错了。

**但 `on` 当默认值，代价落在每一个单人、串行的场景上**：为了记一条动作，先得建区、
cd 进去、再提交。对"一个人、一次一件事"的用法，这是纯粹的仪式；而它防的那个问题，
在那个场景里根本不存在。

用户的判断（2026-10-17）：**默认改成 `off`**，我们用单 agent 顺序推进。

### 这个改动不只是改一个字面量

旧实现里"零值"是偏 safeguard 的：

- `Config.Isolated()` 是 `c.Worktree != WorktreeOff` —— **负向判断**，零值（`""`）落在 `on`；
- `env.isolated()` 同样是 `e.worktree != config.WorktreeOff`；
- `worktreeMode()` 里 `case "", WorktreeOn` 把"没写"归到 `on`。

所以只把 `Read()` 的默认常量从 `WorktreeOn` 换成 `WorktreeOff`，会留下一个自相矛盾的
状态：**配置里没写这个键 → `Read()` 给 `off`，但一个零值 `Config` 仍算隔离**。默认值
翻面必须连**判断方向**一起翻成正向（`== WorktreeOn`），让"没写"在每一处都等于 `off`。

### 边界：不改写历史记录

`docs/worktree-mode.md` / `docs/worktree-hint.md` / `docs/mvp-plan.md` 里写着"默认 on"，
那是**当时的决定记录**，按 kiln 的"只追加、证据保留"原则原样留着；本次翻面另开这一份。
读的人顺着时间看，能看出默认值是**被改过一次**的，而不是一直如此。

## 待办

- [x] 默认值与零值语义一起翻面（`config` 包）。
- [x] 播种模板、`-h` footer、README 同步。
- [x] 测试：默认断言翻面；验证隔离的用例改为**显式** `on`。
- [x] 编译并端到端验证（新仓默认放行、显式 `on` 仍拦截）。

## 落地（apply）

### 代码

- **`internal/config`**：`Read()` 的默认改为 `WorktreeOff`；`worktreeMode()` 的
  `case "", ...` 归到 `WorktreeOff`；`Isolated()` 从负向 `!= WorktreeOff` 翻为正向
  `== WorktreeOn`；两个常量的注释与标志词对调（`on` 现在是“仓库点名要了隔离”）；
  `defaultFile` 播种改为 `"worktree": "off"`。
- **`internal/cli`**：`env.isolated()` 同样翻成正向，与 `Config.Isolated()` 保持同向；
  `-h` footer 改为「`worktree 默认: off（允许在 main 提交）| on（…）`」，把 `off` 放前面
  —— footer 是行为指引，开头的那个词就是默认行为。
- **`README.md`**：配置示例与「工作区模式」那一条改口（说清隔离现在是 **opt-in**：
  要隔离就显式写 `"worktree": "on"`）。
- **本仓 `.kilnconfig.jsonc`**：注释改口，仍显式写 `off`——保留一份“本仓确实选了这个”
  的声明，即使它现在与默认值相同。

### 测试

默认断言三处翻面（`config_test.go`：仓外回退、播种内容、`{}` 解析）。

另外五个 `internal/cli` 用例原先**靠零值拿到隔离**，改后失败——这是对的信号：它们测的是
“仓库点名要了隔离时的护栏”（main 拒绝文案、脏/净分派、`do` 的 stderr 提示、后端配方），
就不该依赖一个恰好也是默认值的零值。改为在 `newWorkspaceEnv` 与
`TestDoTipCarriesTheBackendRecipe` 的内联 `env` 里显式写 `worktree: config.WorktreeOn`。

### 验证

| 项 | 结果 |
| --- | --- |
| `go vet ./...` | 无输出 |
| `go test ./...` | 全绿（改前 5 个 `internal/cli` 用例失败） |
| `go build -o kiln.exe ./cmd/kiln` | 成功；`~/.local/bin/kiln.exe` 是指向它的符号链接，自动生效 |
| `kiln -h` | footer 报 `worktree 默认: off（…）\| on（…）` |
| 空仓（无配置文件）`kiln commit demo ask` | **在 main 直接成功**（`已提交 4bb3630 · ask · demo`）——旧版会结构性拒绝 |
| 同上 `kiln status` | 头行报 `｜worktree off（已开 0 个）` |
| `kiln config path` | 播种的模板含 `"worktree": "off"` 与新注释 |
| 负向对照：显式改 `on` 后再提交 | 被拒，且给出完整配方（`git stash -u` → 建区 → `pop`）+ `--force` 语义说明 |

### 取舍与遗留

- **代价说清楚**：默认 `off` 让新仓库开箱即用，但也意味着**并行 agent 的污染风险要靠
  使用者主动开 `on`**。依据是这个问题只在并行时存在，而默认值服务于多数场景。README
  与 `-h` 仍把两条路都写出来，不藏 `on`。
- **历史文档不改口**：`worktree-mode.md` / `worktree-hint.md` / `mvp-plan.md` 里
  “默认 `on`”原样保留——那条时间线应该看得出默认值被改过一次。
- **`--force` 的语义未变**：它仍是 `on` 下的一次性豁免，不是切换开关；默认翻面只影响
  “没写配置”时护栏是否参与。
