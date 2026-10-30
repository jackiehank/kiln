# issue: path-incident-index — `path --all` 的 I×E：把邻接边建一次索引

- **domain**：cli
- **关联**：follows `cache-precompute`（二次项 A 的落地）；relates-to `path-batch`（批量的形状契约）

## 背景

`cache-precompute` 的 hypothesize/test 定位到两个二次项。B（`FoldEdges` 的 unrelate 全表扫）已随那个 issue 的 apply 落地；剩下 A：`path --all` 与批量 `path` 对每个 issue 调 `incidentEdges`，而它遍历整张边表，于是 O(I×E)。

实测（100k 合成仓，I=16000）：E≈80k 时 4.7 s，E≈240k 时 16.3 s；原型按 issue 建双向索引后降到几十毫秒。本 issue 只做这一项。

## 目标

- 批量 path 一次 O(E) 建邻接，每个 issue 只取自己的度；单 issue 仍走一次扫描（对单个 issue，O(E) 扫描比建两张 map 便宜）。
- 输出逐字节不变：批量里每个 issue 的文档必须与单条 `path <id> --json` 完全相同，含关系顺序与"自关联只出现一次"。
- 内存可接受：100k/240k 边时两张 map 约 20–40 MB，而批量本来就要输出约 50 MB 的 JSON。

## 判据

- 旧/新二进制在 100k 合成仓与真仓上逐字节一致。
- 单测：索引结果与扫描逐条相同（含 self-edge、缺失 id）；带关系仓的批量条目与单条文档 `DeepEqual`。
- 端到端：100k 稀疏 `path --all` ≤ 1 s；E≈240k ≤ 1.5 s。

## 非目标

- 不改 `graph`（它没有 I×E 项）。
- 不把索引落盘或缓存：O(E) 一次构建几十毫秒，重建比失效便宜。
- 不改批量文档的形状或排序。

## 记录

- 2026-10-30：由 `cache-precompute` 的 test 结论拆出，ask 计入。
