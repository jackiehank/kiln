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

## test（2026-10-30）

| 仓（100k 提交，I=16000） | 改前 | 改后 | 倍数 |
| --- | --- | --- | --- |
| plain-100k（E≈80k） | 4 722 ms | **331 ms** | 14× |
| dense2-100k（E≈240k，无 unrelate） | 16 289 ms | **654 ms** | 25× |
| dense-100k（E≈236k，含 unrelate） | 16 074 ms | **799 ms** | 20× |

比 hypothesize 预测的 ~0.7 s 还低：残余只剩折叠 + JSON 编码 + 地板，plain 仓的输出比 dense2 小，所以落在 0.33 s。

逐字节：旧/新二进制在 plain-100k、dense2-100k、dense-100k、真仓上跑 `graph`、`graph --json`、`graph --mermaid`、`graph --issue --depth`、`path <one>`、`path --all --json`、`list`、`status`、`check`、`action log` 共 20 组，全部相同。

## apply（2026-10-30）

- `internal/cli/path.go` 新增 `incidentIndex`：一次 O(E) 建 `out[From]`、`in[To]` 两张 map，`incident(id)` 只取本 issue 的度并按 `(from, to, name)` 排序。
- `pathDocOf` 改为接收 `incident func(string) []graphEdge`：批量传索引，单条传一次扫描的闭包——单 issue 仍是 O(E)，不付建 map 的钱。
- 自关联在 `out`/`in` 各出现一次，`incident` 跳过 `in` 里的自边，与 `incidentEdges` 的"只列一次"一致。
- 新增 `internal/cli/path_index_test.go`：索引 vs 扫描逐条对照（含 self-edge、缺失 id）；带关系仓的批量条目 vs 单条文档 `DeepEqual`（含自关联只出现一次）。

## 非目标

- 不改 `graph`（它没有 I×E 项）。
- 不把索引落盘或缓存：O(E) 一次构建几十毫秒，重建比失效便宜。
- 不改批量文档的形状或排序。

## 记录

- 2026-10-30：由 `cache-precompute` 的 test 结论拆出，ask 计入。
- 2026-10-30：apply 落地邻接索引；100k `path --all --json` 4.7 s → 0.33 s（E≈80k）、16.3 s → 0.65 s（E≈240k），20 组输出逐字节一致。
