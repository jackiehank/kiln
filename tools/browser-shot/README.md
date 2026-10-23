# browser-shot

用真实浏览器（系统 Edge + Playwright）把网页/本地 HTML/`mermaid` 渲染成 PNG。

这是 agent 侧的工具，不是 `kiln` 产品的一部分；它存在的理由见 [docs/browser-shot.md](../../docs/browser-shot.md)。

## 依赖

- Node.js（本机 v24.19.0 验证过）
- `playwright-core`：`%APPDATA%\npm\node_modules\playwright-core`（本机 1.63.0）。脚本会自己找；找不到时可 `set NODE_PATH=%APPDATA%\npm\node_modules`
- 系统 Microsoft Edge（本机 155.0.4283.62）：通过 `channel: 'msedge'` 驱动，**不需要**下载 Playwright 自带的 Chromium
- `mermaid-shot.mjs` 需要一个 `mermaid.min.js` 浏览器包：`$KILN_MERMAID_JS` → `.kiln/mermaid.min.js` → `%LOCALAPPDATA%\Programs` 下的浏览器扩展

## 用法

```powershell
# 截图任意页面（整页）
node tools/browser-shot/shot.mjs https://example.com --out page.png --full-page

# 只截某个元素，等它出现
node tools/browser-shot/shot.mjs .kiln/graph-preview.html --clip svg --wait-for svg --out graph.png

# 等某段文字出现再截（用于 SPA）
node tools/browser-shot/shot.mjs http://127.0.0.1:5173 --wait-for "text:准备就绪" --out app.png

# .mmd 直出 PNG
node tools/browser-shot/mermaid-shot.mjs .kiln/graph-preview.mmd --out graph-mermaid.png --theme default
```

`shot.mjs` 输出一行 JSON：`{"ok":true,"out":"...","title":"...","url":"...","size":{"w":..,"h":..}}`。

## 沙箱注意事项

Chromium 在 DSH 的 `workspace-write` 文件沙箱里起不来：无法创建 profile 锁 / `ProcessSingleton`，无头模式直接 `exit 13`（`0x80000003` STATUS_BREAKPOINT）。所以：

- **agent 调用本工具时必须申请 `danger-full-access`**（`sandbox_permissions`），否则会得到 exit 13；
- **用户自己的终端不受 DSH 沙箱影响**，直接 `node ...` 即可。

## 已知边界

- Windows 上 `msedge.exe --version` 不会打印版本而是直接开窗口——读版本请用 `(Get-Item $edge).VersionInfo.ProductVersion`。
- 无头截图的字体就是你系统的字体；换机器字体不同，同页面的像素会差一点（布局不受影响）。
- `--clip` 传纯 CSS 选择器；要等文字用 `--wait-for "text:..."`，要等元素用纯选择器。
- 只支持 PNG；需要 JPEG 之类的格式请自己转（本机有 ImageMagick）。
