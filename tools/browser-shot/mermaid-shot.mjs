#!/usr/bin/env node
// Render a Mermaid (.mmd) file to PNG through a real browser.
//
//   node mermaid-shot.mjs <input.mmd> [--out out.png] [--theme default|dark|neutral]
//                         [--width px] [--background "#ffffff"] [--scale 2]
//
// Needs a mermaid browser bundle (mermaid.min.js). Resolution order:
//   $KILN_MERMAID_JS, ./.kiln/mermaid.min.js, browser bundles under %LOCALAPPDATA%/Programs
//
// Prints one JSON line: {"ok":true,"out":...,"width":...,"height":...}

import { createRequire } from 'node:module';
import { existsSync, readFileSync, writeFileSync, rmSync, globSync } from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const require = createRequire(import.meta.url);

function loadPlaywright() {
  const candidates = [
    'playwright-core',
    'playwright',
    path.join(process.env.APPDATA || '', 'npm', 'node_modules', 'playwright-core'),
  ];
  for (const c of candidates) {
    try { return require(c); } catch { /* next */ }
  }
  console.error('playwright-core not found; install it or set NODE_PATH');
  process.exit(2);
}

function findMermaid() {
  const candidates = [];
  if (process.env.KILN_MERMAID_JS) candidates.push(process.env.KILN_MERMAID_JS);
  candidates.push(path.resolve('.kiln/mermaid.min.js'));
  const local = path.join(process.env.LOCALAPPDATA || '', 'Programs');
  for (const pattern of [
    '*/resources/app/extensions/*/assets/www/diagram/mermaid.min.js',
    '*/assets/www/diagram/mermaid.min.js',
  ]) {
    try { candidates.push(...globSync(pattern, { cwd: local }).map(p => path.join(local, p))); } catch { /* ignore */ }
  }
  for (const c of candidates) if (c && existsSync(c)) return c;
  console.error('mermaid.min.js not found; set KILN_MERMAID_JS to a local bundle');
  process.exit(2);
}

const argv = process.argv.slice(2);
const opts = { input: null, out: 'mermaid.png', theme: 'default', width: 1600, background: '#ffffff', scale: 2 };
for (let i = 0; i < argv.length; i++) {
  const a = argv[i];
  const next = () => argv[++i];
  if (a === '--out') opts.out = next();
  else if (a === '--theme') opts.theme = next();
  else if (a === '--width') opts.width = Number(next());
  else if (a === '--background') opts.background = next();
  else if (a === '--scale') opts.scale = Number(next());
  else if (a.startsWith('--')) { console.error(`unknown option: ${a}`); process.exit(2); }
  else if (!opts.input) opts.input = a;
  else { console.error(`unexpected argument: ${a}`); process.exit(2); }
}
if (!opts.input || !existsSync(opts.input)) {
  console.error('usage: node mermaid-shot.mjs <input.mmd> [--out out.png]');
  process.exit(2);
}

const esc = (s) => s.replaceAll('&', '&amp;').replaceAll('<', '&lt;').replaceAll('>', '&gt;');
const code = readFileSync(opts.input, 'utf8');
const mermaidPath = findMermaid();
const outPath = path.resolve(opts.out);
const htmlPath = outPath + '.render.html';
writeFileSync(htmlPath, `<!doctype html>
<html><head><meta charset="utf-8"><title>mermaid render</title>
<style>html,body{margin:0;padding:0;background:${opts.background};} #out{padding:8px;display:inline-block;}</style>
</head><body><div id="out"><pre class="mermaid">${esc(code)}</pre></div>
<script src="${pathToFileURL(mermaidPath).href}"></script>
<script>mermaid.initialize({ startOnLoad: true, theme: '${opts.theme}' });</script>
</body></html>`);

const { chromium } = loadPlaywright();
const browser = await chromium.launch({ channel: 'msedge', headless: true });
try {
  const page = await browser.newPage({ viewport: { width: opts.width, height: 1200 }, deviceScaleFactor: opts.scale });
  page.on('pageerror', (e) => console.error('PAGEERROR:', e.message));
  await page.goto(pathToFileURL(htmlPath).href);
  await page.waitForSelector('#out svg', { timeout: 30000 });
  await page.waitForTimeout(300);
  // mermaid sizes the svg to its container (width:100%); pin it to the viewBox
  // so the screenshot gets the diagram's natural size instead of the viewport width.
  const size = await page.evaluate(() => {
    const svg = document.querySelector('#out svg');
    svg.style.maxWidth = 'none';
    const vb = svg.viewBox.baseVal;
    if (vb && vb.width && vb.height) {
      svg.setAttribute('width', vb.width);
      svg.setAttribute('height', vb.height);
    }
    const r = svg.getBoundingClientRect();
    return { w: Math.ceil(r.width), h: Math.ceil(r.height) };
  });
  await page.setViewportSize({ width: size.w + 32, height: size.h + 32 });
  const el = await page.$('#out svg');
  const box = await el.boundingBox();
  await page.screenshot({ path: outPath, clip: box, omitBackground: false });
  console.log(JSON.stringify({ ok: true, out: outPath, width: Math.ceil(box.width), height: Math.ceil(box.height), mermaid: mermaidPath }));
} finally {
  await browser.close();
  rmSync(htmlPath, { force: true });
}
