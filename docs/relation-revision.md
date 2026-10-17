# issue: relation-revision — 关系写错了怎么改：改历史、追加撤销，还是把关系移出动作历史

- **domain**：design
- **关联**：derived-from `relation-types`、relates-to `evidence-fluidity`

## 背景

`kiln` 的关系是**跨该 issue 全部动作 commit 的并集**（`internal/engine/engine.go:71-75`）：

```go
p.DerivedFrom = appendUnique(p.DerivedFrom, a.DerivedFrom...)
p.RelatesTo   = appendUnique(p.RelatesTo, a.RelatesTo...)
p.Blocks      = appendUnique(p.Blocks, a.Blocks...)
```

→ **只增不减**。一条关系一旦写进任何一次动作，就永久留在 `path` 的关系摘要里；`relation-types`
的非目标也写明「不重写已有提交：现有的 derived-from / relates-to 用法继续有效」，README 的取向是
「不改写历史——动作就是提交，撤销 / 回滚 / 跨分支复用全用 git 自己的本事」（README:284）。

真实代价（wsl-agent 的 issue 梳理，2026-10-17）：先用 `relates-to` 记了两条弱关联，随后发现真正的
语义是 `derived-from` + `blocks`；因为不能删，**只能追加精确的边，让冗余的 `relates-to` 永久留着**。
同一次还撞上两个相关缺口：

1. **反向推导没有实现**。`relation-types` 的结论是「声明一侧、读侧自动推导反向」，但代码里没有：
   `relationLines`（`engine.go:302-313`）只画本 issue 自己存的那三族。→ 要让两侧视图都完整，就得
   **写两遍**（本次确实每条边都写了两次：`A spawns B` + `B derived-from A`）；日后要修正也是两遍。
2. **写错目标无人管**。`check` 只校验 `derived-from` 的父存在、`spawns` 的子存在；`blocks` /
   `relates-to` / `supersedes` 指向不存在的 issue 也不报。

## 要回答的问题

1. **取向先定**：关系是**证据**（不可改，与 `evidence-fluidity` 同题），还是**当前事实的描述**
   （可改，且改它不该碰动作内容）？
   > 我的判断是后者：关系不记录"某一步发生了什么"，只描述"现在这两条 issue 是什么关系"。
   > 这也正是它与 `evidence-fluidity` 的**边界**：动作内容（diff、标题、证据）可以不可改，
   > 关系却应该可改——两者不该被同一条约定绑在一起。
2. **若不改历史，机制是什么**：
   - **追加型撤销**：新增 trailer（如 `unrelates-to:` / `revises-relation:`），读侧按最新动作覆盖；
   - **关系清单**：把关系移出动作历史，改成 issue 级、可编辑的元数据（文件或 `kiln relate` 命令），
     动作 commit 只负责内容；
   - **同键整组后写覆盖**（last-wins）：语义变化大——`path` 里"每一步看到的关系"会不同，而
     `path` 现在呈现的是并集。
3. **反向推导要不要实现**：实现后声明一侧就够、修正也只改一侧（`relation-types` 已定"读侧推导"，
   但**没落地**）。不实现就得双侧维护，且两侧可能不一致（现在就没有任何一致性校验）。
4. **历史改写允不允许**：README 说"用 git 自己的本事"——那就意味着"关系写错 → rebase/amend"是
   官方答案。但这与"关系是长期资产"的期望冲突吗？jj 用户（本仓就是 jj）改历史是日常操作，
   git 用户则危险得多。**要不要对两侧给不同建议？**
5. **检出**：关系若可撤销 / 修订，`path` 要不要显示"这条边后来被撤销过"（以及谁撤的）？
6. **与 `check` 的互动**：关系可改之后，"目标存在"校验要不要补齐到 5 个键（现在只查两族）？

## 候选

| 方案 | 形态 | 代价 |
| --- | --- | --- |
| **A** 只立约定 | 关系写错就改历史（`jj describe` / `git rebase`） | 零机制；对 jj 是日常，对 git 危险；且违背"不重写"的默认叙事 |
| **B** 追加型撤销 | `unrelates-to:` / `revises-relation:`，读侧覆盖 | 字段增殖；"撤销的撤销"要定义 |
| **C** 关系移出动作历史 | issue 级关系清单（文件或 `kiln relate`），可改 | 最大改动：关系不再随动作留痕，与 trailer 模型分道 |
| **D** last-wins | 同键后写的动作整组覆盖 | 语义变化大；与 `path` 的"每步并集"呈现冲突 |
| **E** 维持 | 现状：并集、只增不减 | 零成本；冗余边永久留存（已在真实项目里发生） |

> 倾向（待验证）：**先答 1 的取向，再在 B / C 里选**。B 最贴现有模型（trailer 驱动、可审计、
> 与 `action-diff` 同族），C 最干净但动"关系从哪来"的根；A 依赖 `evidence-fluidity` 的结论，
> 且与 README 的"不改写历史"叙事直接相关。

## 非目标

- 不讨论动作内容（diff / 标题 / 证据）能不能改——那是 `evidence-fluidity`。
- 不改关系词表与渲染（分别是 `relation-vocabulary` / `relation-rendering`）。
- 不在本 issue 里决定 `part-of` 的落地（`relation-types` 的遗留，见 `silent-flag-drop`）。

## 证据

- `internal/engine/engine.go:71-75`：`appendUnique` 的并集语义（只增不减）。
- `internal/engine/engine.go:302-313`：`relationLines` 只画本 issue 存的三族——**反向推导确实没实现**。
- `internal/engine/engine.go` `checkIssue`：只校验 derived-from / spawns 的目标存在。
- `docs/relation-types.md` 结论 1（"声明一侧、读侧推导反向"）与非目标（"不迁移历史"）。
- `README.md:284`：不改写历史，撤销 / 回滚交给 git / jj。
- wsl-agent 实例（2026-10-17）：`relates-to` 无法退场，只能追加 `derived-from` / `blocks`；
  每条边都要两侧各写一次（`A spawns B` + `B derived-from A`）。
