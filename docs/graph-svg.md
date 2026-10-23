# issue: graph-svg — 没有无头浏览器时，怎么把关系图变成一张能看的图

- **domain**：cli
- **关联**：relates-to `graph-export`、relates-to `graph-normalize`

## 背景

`graph-export` 落了 `--mermaid`（2026-10-23，commit `847f92c`），但要"看看效果"就得有人渲染它。本环境没有可用的无头浏览器：

- Edge `--headless=new` / `--headless` 在 DSH 沙箱里直接 `exit 13` / `0x80000003`（STATUS_BREAKPOINT），`--no-sandbox`、`--disable-gpu-sandbox`、`--disable-crashpad`、`--disable-breakpad`、`--dump-dom`、`--screenshot`、`Start-Process` 全部同一结果；`--version` 都起不来。
- 没有 Playwright / Puppeteer 的浏览器缓存，bundled node `node_modules` 是空的；没有 LibreOffice，没有 `dot`。
- 出站网络整体不可用：shell 里 `https://cdn.jsdelivr.net`、`registry.npmjs.org`、`unpkg`、`cdnjs` 全部 TLS 失败；harness 的 `web_fetch` 走 `api.deepseek.com/v1/tools/fetch` 返回 404。所以 kroki / mermaid.ink 这类在线渲染也用不了。

结果是：`--mermaid` 的文本能写出来，但**没有任何一步把它变成像素**——agent 无法用 `read_image` 自查图形输出，用户也可能在不联网的机器上打不开依赖 CDN 的预览页。

## 问题

1. 图形输出（ascii / mermaid / 将来的 json）怎么得到**可验证的渲染结果**，且不依赖网络、不依赖安装？
2. 渲染器放哪：产品里（`kiln graph --svg`）还是只做 agent 侧的验证管线（脚本把文本转 PNG）？
3. 如果自绘，布局要做到什么程度：分层 LR、节点不重叠、边与标签不压字、48 节点仍可读？
4. `--mermaid`、`--svg`、`--json` 三种出口如何分工（渲染交给谁、中间产物给谁）？

## 候选

| 方案 | 做法 | 依赖 | 评价 |
| --- | --- | --- | --- |
| **A** 产品出 SVG | `graph --svg` 直接从 `FoldEdges` 画 SVG：分层 LR、圆角框、有向箭头、对称虚线、标签截断 | 无（纯 Go 字符串） | SVG 浏览器/画图软件直接开；再配本机 `magick` 转 PNG 即可自查。布局代码要自己写 |
| **B** agent 侧管线 | 只写脚本（Go/Python）把 ascii/mermaid 转 SVG/PNG，不进产品 | 无 | 快，但"预览能力"只属于本机 agent，用户拿不到 |
| **C** Pillow 直接画 PNG | 用 bundled Python + Pillow 画节点/箭头 | Pillow | 只能出位图、文字换行/箭头质量一般；同样只适合验证 |
| **D** 在线渲染 | kroki / mermaid.ink / GitHub | 网络 | 本环境完全不可达；用户侧也不保证 |
| **E** 换无头方案 | Playwright、Puppeteer、Chromium、WebView2、Office COM | 安装 + 沙箱 | 无缓存、安装要网络；COM 重且沙箱风险高 |
| **F** 办公套件渲染 | LibreOffice 把 HTML/SVG 转 PNG | LibreOffice | 本机没有；装了也不该成为产品依赖 |

## 验收（草案）

- **E1**：`kiln graph --svg` 与 `--format=svg` 同义；默认 `ascii` 逐字节不变。
- **E2**：节点 = issue（`id (state)`），边 = `FoldEdges` 的同一份结果；有向画箭头，对称画虚线，目标缺失标注 `(missing)`。
- **E3**：SVG 自包含（无 `script`、无外链、无字体下载），`magick` 一转就是 PNG，`read_image` 能看清 48 节点图的节点名与边标签。
- **E4**：布局确定性：同仓同文本、同像素；层内按 id 排序，不因 map 顺序抖动。
- **E5**：大图策略明确：层间距/列宽给出上限，长 id 截断加省略号（`path` 里看全名）。
- **E6**：`go build ./...` / `go test ./...` 全过；最小仓 + 真仓各出一张图。

## 非目标

- 不引入布局库（dagre / graphviz）或字体文件；
- 不做交互（缩放、点击、折叠）；
- 不改 `--mermaid` 与 `--json` 的形态；
- 不做"把 PNG 提交进仓库"的截图快照。

