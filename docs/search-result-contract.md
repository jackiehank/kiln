# issue: search-result-contract — 搜索行要能被 UI 直接渲染（rank + hit 的 total/date）

- **domain**：cli
- **关联**：relates-to `issue-list-scaling`（`list --search` 在这里落地）；relates-to `json-commit-hash`（`hit` 对象形态）；relates-to `machine-readable-output`（JSON 契约）

## 背景

`kiln list --search` 已经能按 id / 当前标题 / 动作 subject 做字面量 AND 搜索，`--json` 也给 `hit` 对象。下游 `dsh-kiln` 插件正在把搜索从「浏览器内存匹配」换成「调 `kiln list --search --json` 后只渲染」（B 方案；220ms 的端到端延迟已实测可接受）。

渲染需要三样数据，现在没有：

1. **相关度顺序**：插件现有排序是 id 首词前缀 > id 命中 > 标题命中 > 单条动作命中 > 词分散，再按最近动作；kiln 现在固定 id 字典序。UI 若自己再排一次，就又有两套算法。
2. **`hit.total`**：结果行要显示「命中第 n/m 条」；`hit` 只有 ordinal，没有 m。
3. **`hit.date`**：结果预览要显示命中动作的日期；现在只能再查 `path`。

本 issue 只补这三样与它们的顺序契约；不改匹配语义，也不做 UI 的高亮/交互。

## 需求

1. **rank**：`list --search ... --json` 的每个 `issues[]` 行必须有整数 `rank`（0–4），且 `issues[]` 按 rank 排序。不带 `--search` 的 `list` 不输出 `rank`（缺省即省略），顺序仍按 id。
2. **rank 的定义**（与 dsh-kiln 现有搜索一致；词序按查询串）：
   - `0`：issue id（不区分大小写）以**第一个词**开头；
   - `1`：issue id 含**全部**词；
   - `2`：当前标题（`issues[].title` 的来源）含全部词；
   - `3`：至少一条动作 subject 含全部词；
   - `4`：整体命中，但以上都不满足（词分散在多个字段/多条动作）。
   0 优先于 1；桶内再按 tie-break。
3. **tie-break**：同 rank 按末条动作日期降序；日期相同或缺失时按 issue id 升序。日期按时间比较，不按字符串字典序。
4. **hit 元数据**：`hit` 继续只在「id + 标题解释不了命中，且某一条动作含全部词」时出现；对象保持 `{ordinal, action, commit, subject}`，新增：
   - `total`：该 issue 的动作总数（整数，≥1）；
   - `date`：命中动作的日期，格式与 `path --json` 的 `actions[].date` 一致。
5. **文本与 JSON 同序**：`list --search` 的文本视图按同样的 rank/tie-break 排序；除顺序外文本布局不变（仍带 `← subject`）。
6. **不变的部分**：匹配字段/分词/AND/字面量/大小写、`hit` 的触发规则、全仓范围、与 `--domain/--tag/--action/--status` 的 AND 叠加、空结果 exit 0 + stderr、空查询 exit 2、`count`/`filter` 回显、非搜索的 `list`/`status`/`graph`/`path`/`show` 输出。
7. **文档**：`docs/machine-readable-output.md` 的 `list` 节与 README 的 `--search` 说明补 rank、顺序与 hit 新字段；说明「只有 `--search` 改变顺序，普通 list 仍按 id」。

## 测试

只列必须覆盖的用例，具体测试写法由实现方决定：

- **rank 分档**：一个 fixture 覆盖 0/1/2/3/4 各至少一条；断言每行 `rank` 与 `issues[]` 顺序。
- **tie-break**：同 rank 的两条（日期不同；日期相同）→ 日期降序、再 id 升序；日期跨时区仍按时间。
- **hit**：命中早期动作时 `ordinal/action/commit/subject/total/date` 全对；`ordinal` 从 1 起；`total == 该 issue 动作数`。
- **hit 抑制**：id+标题已含全部词 → 无 `hit`；词分散在多条动作 → 无 `hit`（保持现状）。
- **文本/JSON 同序**：同一 query 抓文本行序与 JSON `issues[]`，逐条相等。
- **非搜索不变**：`list --json`（无 `--search`）与改前逐字节相同；`list` 文本逐行相同；`rank` 不出现。
- **组合**：`--search` + `--domain/--tag/--action/--status`，断言在过滤后的集合内按 rank 排序。
- **边界**：大小写、多词 AND、字面量、超长 subject（>200 字符）、无结果、空查询。
- **真仓冒烟**：在本仓跑一个已知多档的 query，记录 JSON 与文本顺序；再跑一个 `list --json` 对拍。

## 验收

- `kiln list --search <q> --json`：每行有 `rank`，`hit` 有 `total`/`date`；`issues[]` 按契约排序。
- `kiln list --search <q>`：文本顺序与 JSON 相同，`← subject` 保留。
- `kiln list --json`（无 search）输出与现状逐字节相同；`list` 文本相同。
- `gofmt` / `go vet` / `go test ./...` 全过；新增测试覆盖上面的用例。
- README 与 `docs/machine-readable-output.md` 同步。
- 真仓 dogfood：搜索排序符合 rank 契约；非搜索输出无 diff。

## 非目标

- 不改 `--search` 的匹配语义与 `hit` 触发规则；
- 不输出高亮区间、不实现 UI 的 filter/search 切换；
- 不加分页 / `--limit` / `--offset`；
- 不新增独立 `search` 命令；
- 不改 `status` / `graph` / `path` / `show` 的输出。

## 备注

- 需求来源：下游 `dsh-kiln` 插件的搜索接入（B 方案）。插件只保留高亮与交互，匹配/顺序/hit 全部以 kiln 为准；本 issue 是「一套算法」的前置。
- `issue-list-scaling` 是 `list --search` 的落地 issue；`json-commit-hash` 定了 `hit` 的对象形态；本 issue 只增量补 UI 渲染需要的元数据与顺序。
- 排序变化只发生在 `--search`；普通 `list` 的 id 序不变，避免影响现有 CI/golden。
