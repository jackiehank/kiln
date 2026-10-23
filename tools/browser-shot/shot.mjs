#!/usr/bin/env node
// Screenshot any URL / local HTML page through a real browser.
// Uses playwright-core driving the locally installed Microsoft Edge.
//
//   node shot.mjs <target> [options]
//
//   <target>            http(s) URL or a local file path (converted to file://)
// Options:
//   --out <path>        output PNG (default: shot.png)
//   --width <px>        viewport width (default 1440)
//   --height <px>       viewport height (default 900)
//   --settle <ms>       extra wait after load (default 600)
//   --wait-for <sel>    wait: CSS selector, or text:<substring>
//   --clip <sel>        screenshot only the element matching this CSS selector
//   --full-page         capture the whole scrollable page
//   --channel <name>    playwright browser channel (default msedge)
//   --headful           show the browser window
//   --timeout <ms>      per-step timeout (default 30000)
//
// Prints one JSON line: {"ok":true,"out":...,"title":...,"url":...,"size":{w,h}}

import { createRequire } from 'node:module';
import { existsSync } from 'node:fs';
import path from 'node:path';
import { pathToFileURL } from 'node:url';

const require = createRequire(import.meta.url);

function loadPlaywright() {
  const candidates = [
    'playwright-core',
    'playwright',
    path.join(process.env.APPDATA || '', 'npm', 'node_modules', 'playwright-core'),
    path.join(process.env.ProgramFiles || 'C:/Program Files', 'nodejs', 'node_modules', 'playwright-core'),
  ];
  for (const c of candidates) {
    try {
      return require(c);
    } catch {
      /* try next */
    }
  }
  console.error('playwright-core not found; install it or set NODE_PATH to a directory containing it');
  process.exit(2);
}

function parseArgs(argv) {
  const opts = {
    target: null,
    out: 'shot.png',
    width: 1440,
    height: 900,
    settle: 600,
    waitFor: null,
    clip: null,
    fullPage: false,
    channel: 'msedge',
    headless: true,
    timeout: 30000,
  };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    const next = () => argv[++i];
    switch (a) {
      case '--out': opts.out = next(); break;
      case '--width': opts.width = Number(next()); break;
      case '--height': opts.height = Number(next()); break;
      case '--settle': opts.settle = Number(next()); break;
      case '--wait-for': opts.waitFor = next(); break;
      case '--clip': opts.clip = next(); break;
      case '--full-page': opts.fullPage = true; break;
      case '--channel': opts.channel = next(); break;
      case '--headful': opts.headless = false; break;
      case '--timeout': opts.timeout = Number(next()); break;
      case '-h':
      case '--help': opts.help = true; break;
      default:
        if (a.startsWith('--')) { console.error(`unknown option: ${a}`); process.exit(2); }
        if (!opts.target) opts.target = a;
        else { console.error(`unexpected argument: ${a}`); process.exit(2); }
    }
  }
  return opts;
}

function toUrl(target) {
  if (/^[a-z][a-z0-9+.-]*:\/\//i.test(target) || target.startsWith('data:') || target.startsWith('about:')) {
    return target;
  }
  const abs = path.resolve(target);
  if (!existsSync(abs)) {
    console.error(`no such file: ${abs}`);
    process.exit(2);
  }
  return pathToFileURL(abs).href;
}

const opts = parseArgs(process.argv.slice(2));
if (opts.help || !opts.target) {
  console.error('usage: node shot.mjs <target> [--out shot.png] [--width 1440] [--height 900]');
  console.error('       [--wait-for css-selector|text:foo] [--clip css-selector] [--full-page] [--settle ms]');
  process.exit(opts.help ? 0 : 2);
}

const { chromium } = loadPlaywright();
const url = toUrl(opts.target);
const browser = await chromium.launch({ channel: opts.channel, headless: opts.headless });
try {
  const page = await browser.newPage({ viewport: { width: opts.width, height: opts.height } });
  page.setDefaultTimeout(opts.timeout);
  await page.goto(url, { waitUntil: 'load', timeout: opts.timeout });
  if (opts.waitFor) {
    if (opts.waitFor.startsWith('text:')) {
      await page.getByText(opts.waitFor.slice(5), { exact: false }).first().waitFor();
    } else {
      await page.waitForSelector(opts.waitFor);
    }
  }
  if (opts.settle > 0) await page.waitForTimeout(opts.settle);
  const shotOpts = { path: path.resolve(opts.out) };
  if (opts.clip) {
    const el = await page.$(opts.clip);
    if (!el) throw new Error(`clip selector matched nothing: ${opts.clip}`);
    shotOpts.clip = await el.boundingBox();
    if (!shotOpts.clip) throw new Error(`clip element has no box: ${opts.clip}`);
  } else if (opts.fullPage) {
    shotOpts.fullPage = true;
  }
  await page.screenshot(shotOpts);
  const size = await page.evaluate(() => ({
    w: document.documentElement.scrollWidth,
    h: document.documentElement.scrollHeight,
  }));
  console.log(JSON.stringify({ ok: true, out: shotOpts.path, title: await page.title(), url, size }));
} finally {
  await browser.close();
}
