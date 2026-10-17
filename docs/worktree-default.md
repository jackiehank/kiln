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

- [ ] 默认值与零值语义一起翻面（`config` 包）。
- [ ] 播种模板、`-h` footer、README 同步。
- [ ] 测试：默认断言翻面；验证隔离的用例改为**显式** `on`。
- [ ] 编译并端到端验证（新仓默认放行、显式 `on` 仍拦截）。
