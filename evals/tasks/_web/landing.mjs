// Hidden Playwright checks shared by the web-landing-* tasks. usage: node landing.mjs <baseUrl> <json config>
// config: {sections:[testids], items:{testid:min}, priced:[testids]}; weights below sum to 82 (build/strict added by verify).
// If EVAL_SCREENSHOT_DIR is set, writes 390.png and 1440.png (full page) there.
import { chromium } from "playwright";
import { AxeBuilder } from "@axe-core/playwright";
import fs from "node:fs";

const url = process.argv[2];
const cfg = JSON.parse(process.argv[3] || "{}");
const checks = [];
const consoleErrors = [];
const external = [];
let browser;

async function open(width = 1440, height = 900) {
  const ctx = await browser.newContext({ viewport: { width, height } });
  const page = await ctx.newPage();
  page.setDefaultTimeout(3000);
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text().slice(0, 200)); });
  page.on("pageerror", (e) => consoleErrors.push("pageerror: " + String(e).slice(0, 200)));
  page.on("request", (r) => {
    const u = r.url();
    if (!u.startsWith("data:") && !u.startsWith("blob:") && new URL(u).hostname !== "127.0.0.1") external.push(u.slice(0, 120));
  });
  await page.goto(url, { waitUntil: "load" });
  // Scroll through the page so lazy content and animations settle before measuring.
  await page.evaluate(async () => {
    for (let y = 0; y < document.body.scrollHeight; y += 600) { window.scrollTo(0, y); await new Promise((r) => setTimeout(r, 40)); }
    window.scrollTo(0, 0);
  });
  return { ctx, page };
}

async function check(name, weight, fn, opts = {}) {
  let t;
  try {
    t = await open(opts.width, opts.height);
    let res = await fn(t.page);
    if (res === true || res === undefined) res = { frac: 1 };
    if (res === false) res = { frac: 0 };
    checks.push({ name, weight, score: Math.round(weight * res.frac * 100) / 100, pass: res.frac >= 0.999, detail: res.detail || "" });
  } catch (e) {
    checks.push({ name, weight, score: 0, pass: false, detail: String(e.message || e).split("\n")[0].slice(0, 220) });
  } finally {
    if (t) await t.ctx.close();
  }
}

browser = await chromium.launch();

// Vacuous checks (overflow, axe, alt text, ...) pass on an empty page, so they only score once the page has content.
let hasContent = false;
{
  const t = await open();
  let n = 0;
  for (const id of cfg.sections) if (await t.page.getByTestId(id).first().isVisible().catch(() => false)) n++;
  hasContent = n >= Math.ceil(cfg.sections.length / 2);
  await t.ctx.close();
}
const gated = (fn) => async (p) => (hasContent ? fn(p) : { frac: 0, detail: "skipped: page has no real content yet" });

await check("required sections present and visible", 14, async (p) => {
  const missing = [];
  for (const id of cfg.sections) if (!(await p.getByTestId(id).first().isVisible().catch(() => false))) missing.push(id);
  return { frac: (cfg.sections.length - missing.length) / cfg.sections.length, detail: missing.length ? "missing: " + missing.join(", ") : "" };
});

await check("required items (cards, tiers, models, FAQ entries, prices)", 6, async (p) => {
  const problems = [];
  const entries = Object.entries(cfg.items);
  for (const [id, min] of entries) {
    const n = await p.getByTestId(id).count();
    if (n < min) problems.push(`${id}: ${n} < ${min}`);
  }
  for (const id of cfg.priced || []) {
    const texts = await p.getByTestId(id).allTextContents();
    if (!texts.length || !texts.every((x) => /[$€£]\s?\d|\d\s?(€|£|\$)/.test(x))) problems.push(`${id}: every item needs a price like $29`);
  }
  const total = entries.length + (cfg.priced || []).length;
  return { frac: Math.max(0, (total - problems.length) / total), detail: problems.join("; ") };
});

await check("semantic headings: one h1, no skipped levels, each section headed", 8, async (p) => {
  const info = await p.evaluate((sections) => {
    const hs = [...document.querySelectorAll("h1,h2,h3,h4,h5,h6")].map((h) => Number(h.tagName[1]));
    let skips = 0;
    for (let i = 1; i < hs.length; i++) if (hs[i] > hs[i - 1] + 1) skips++;
    const unheaded = sections.filter((id) => { const el = document.querySelector(`[data-testid="${id}"]`); return el && !el.querySelector("h1,h2,h3,h4,h5,h6"); });
    return { h1: hs.filter((x) => x === 1).length, first: hs[0], skips, unheaded };
  }, cfg.sections);
  let frac = 0;
  if (info.h1 === 1 && info.first === 1) frac += 0.4;
  if (info.skips === 0) frac += 0.4;
  if (info.unheaded.length === 0) frac += 0.2;
  return { frac, detail: `h1=${info.h1} skips=${info.skips} unheaded=${info.unheaded.join(",")}` };
});

await check("images and svgs have text alternatives", 4, gated(async (p) => {
  const bad = await p.evaluate(() => {
    const imgs = [...document.querySelectorAll("img")].filter((i) => i.getAttribute("alt") === null && i.getAttribute("role") !== "presentation");
    const svgs = [...document.querySelectorAll('svg[role="img"]')].filter((s) => !s.getAttribute("aria-label") && !s.querySelector("title") && !s.getAttribute("aria-labelledby"));
    return imgs.length + svgs.length;
  });
  return { frac: bad === 0 ? 1 : 0, detail: bad ? `${bad} without alternative` : "" };
}));

await check("lead form: at least 3 fields, all labelled", 6, async (p) => {
  const r = await p.evaluate(() => {
    const f = document.querySelector('[data-testid="lead-form"]');
    if (!f) return { n: 0, bad: 0 };
    const ctrls = [...f.querySelectorAll("input,select,textarea")].filter((c) => !["hidden", "submit", "button"].includes(c.type));
    const name = (c) => {
      if (c.getAttribute("aria-label")) return true;
      const lb = c.getAttribute("aria-labelledby");
      if (lb && lb.split(" ").some((id) => document.getElementById(id)?.textContent.trim())) return true;
      if (c.id && document.querySelector(`label[for="${CSS.escape(c.id)}"]`)?.textContent.trim()) return true;
      return !!c.closest("label")?.textContent.trim();
    };
    return { n: ctrls.length, bad: ctrls.filter((c) => !name(c)).length };
  });
  if (r.n < 3) return { frac: 0, detail: `${r.n} fields (need >= 3) or no lead-form` };
  return { frac: (r.n - r.bad) / r.n, detail: r.bad ? `${r.bad} unlabelled` : "" };
});

await check("form validation: empty submit shows errors", 6, async (p) => {
  await p.getByTestId("form-submit").scrollIntoViewIfNeeded();
  await p.getByTestId("form-submit").click();
  let frac = 0;
  const detail = [];
  if (await p.getByTestId("form-error").first().isVisible().catch(() => false)) frac += 0.6; else detail.push("no visible form-error");
  if ((await p.getByTestId("form-success").count()) === 0) frac += 0.2; else detail.push("success shown for empty form");
  if ((await p.locator('[data-testid="lead-form"] [aria-invalid="true"]').count()) > 0) frac += 0.2; else detail.push("no aria-invalid field");
  return { frac, detail: detail.join("; ") };
});

await check("form success: valid submit shows confirmation", 4, async (p) => {
  const form = p.getByTestId("lead-form");
  const ctrls = await form.locator("input,select,textarea").elementHandles();
  for (const c of ctrls) {
    const info = await c.evaluate((el) => ({ tag: el.tagName, type: el.type, name: (el.name || el.id || "").toLowerCase() }));
    if (info.tag === "SELECT") { const opts = await c.evaluate((el) => [...el.options].map((o) => o.value).filter(Boolean)); if (opts.length) await c.selectOption(opts[opts.length - 1]); }
    else if (info.type === "checkbox") await c.check();
    else if (info.type === "radio") await c.check();
    else if (info.type === "email" || info.name.includes("mail")) await c.fill("test@example.com");
    else if (info.type === "tel" || info.name.includes("phone")) await c.fill("0612345678");
    else if (info.type === "number") await c.fill("3");
    else if (["hidden", "submit", "button"].includes(info.type)) continue;
    else await c.fill("Test value that is long enough");
  }
  await p.getByTestId("form-submit").click();
  await p.getByTestId("form-success").waitFor({ state: "visible" });
});

for (const [w, label] of [[390, "mobile 390px"], [1440, "desktop 1440px"]]) {
  await check(`no horizontal overflow (${label})`, 4, gated(async (p) => {
    const r = await p.evaluate(() => ({ sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth }));
    return { frac: r.sw <= r.cw + 1 ? 1 : 0, detail: r.sw > r.cw + 1 ? `scrollWidth ${r.sw} > ${r.cw}` : "" };
  }), { width: w, height: 900 });
}

await check("accessibility (axe): no serious/critical violations", 8, gated(async (p) => {
  const res = await new AxeBuilder({ page: p }).analyze();
  const bad = res.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  return { frac: bad.length === 0 ? 1 : bad.length <= 2 ? 0.5 : 0, detail: bad.map((v) => `${v.id}(${v.nodes.length})`).join(", ") };
}));

await check("no placeholder copy (lorem ipsum, TODO, your text here)", 3, gated(async (p) => {
  const text = await p.evaluate(() => document.body.innerText);
  const m = text.match(/lorem ipsum|dolor sit amet|placeholder text|your (company|text|name) here|\bTODO\b|coming soon/i);
  return { frac: m ? 0 : 1, detail: m ? `found "${m[0]}"` : "" };
}));

await check("real content: at least 500 words of copy", 5, async (p) => {
  // textContent also counts collapsed FAQ answers, which are real copy on the page.
  const words = await p.evaluate(() => document.body.textContent.split(/\s+/).filter(Boolean).length);
  return { frac: words >= 500 ? 1 : words >= 250 ? 0.5 : 0, detail: `${words} words` };
});

await check("FAQ items toggle (details/summary or aria-expanded buttons)", 3, async (p) => {
  const first = p.getByTestId("faq-item").first();
  const isDetails = await first.evaluate((el) => el.tagName === "DETAILS" || !!el.querySelector("details,summary"));
  if (isDetails) {
    const d = (await first.evaluate((el) => el.tagName)) === "DETAILS" ? first : first.locator("details").first();
    const before = await d.evaluate((el) => el.open);
    await d.locator("summary").first().click();
    return { frac: (await d.evaluate((el) => el.open)) !== before ? 1 : 0 };
  }
  const btn = first.locator("button,[role=button],[aria-expanded]").first();
  const before = await btn.getAttribute("aria-expanded");
  await btn.click();
  const after = await btn.getAttribute("aria-expanded");
  return { frac: before !== null && after !== before ? 1 : 0, detail: "button must toggle aria-expanded" };
});

{
  const n = external.length;
  checks.push({ name: "no external network requests (fonts, images, scripts)", weight: 3, score: n === 0 && hasContent ? 3 : 0, pass: n === 0 && hasContent, detail: n ? external.slice(0, 2).join(" | ") : "" });
  const c = consoleErrors.length;
  checks.push({ name: "no console errors or page errors", weight: 4, score: c === 0 && hasContent ? 4 : 0, pass: c === 0 && hasContent, detail: c ? consoleErrors.slice(0, 2).join(" | ") : "" });
}

const shotDir = process.env.EVAL_SCREENSHOT_DIR;
if (shotDir) {
  fs.mkdirSync(shotDir, { recursive: true });
  for (const w of [390, 1440]) {
    const t = await open(w, 900);
    await t.page.waitForTimeout(400);
    await t.page.screenshot({ path: `${shotDir}/${w}.png`, fullPage: true });
    await t.ctx.close();
  }
}

await browser.close();
console.log(JSON.stringify({ checks }));
