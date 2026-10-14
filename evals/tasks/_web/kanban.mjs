// Hidden Playwright checks for web-kanban. usage: node kanban.mjs <baseUrl> <json extra>
// Prints one JSON line {"checks":[{name,weight,score,pass,detail}]}. Weights sum to 100 (build/strict are added by verify.py).
import { chromium } from "playwright";
import { AxeBuilder } from "@axe-core/playwright";

const url = process.argv[2];
const checks = [];
const consoleErrors = [];
let browser;

async function newPage(viewport = { width: 1280, height: 900 }) {
  const ctx = await browser.newContext({ viewport });
  const page = await ctx.newPage();
  page.setDefaultTimeout(3000);
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text().slice(0, 200)); });
  page.on("pageerror", (e) => consoleErrors.push("pageerror: " + String(e).slice(0, 200)));
  page.on("dialog", (d) => { consoleErrors.push("unexpected dialog: " + d.type()); d.dismiss(); });
  await page.goto(url);
  await page.getByTestId("board").waitFor();
  return { ctx, page };
}

async function check(name, weight, fn) {
  let res;
  const t = await newPage().catch((e) => ({ err: e }));
  if (t.err) {
    checks.push({ name, weight, score: 0, pass: false, detail: "page failed to load: " + String(t.err).slice(0, 150) });
    return;
  }
  try {
    res = await fn(t.page, t.ctx);
  } catch (e) {
    res = { frac: 0, detail: String(e.message || e).split("\n")[0].slice(0, 220) };
  }
  if (res === true || res === undefined) res = { frac: 1 };
  if (res === false) res = { frac: 0 };
  const frac = res.frac ?? (res.pass ? 1 : 0);
  checks.push({ name, weight, score: Math.round(weight * frac * 100) / 100, pass: frac >= 0.999, detail: res.detail || "" });
  await t.ctx.close();
}

const col = (p, id) => p.getByTestId(`column-${id}`);
const titles = (p, id) => col(p, id).getByTestId("card-title").allTextContents();
const add = async (p, id, text, enter = false) => {
  await p.getByTestId(`add-input-${id}`).fill(text);
  if (enter) await p.getByTestId(`add-input-${id}`).press("Enter");
  else await p.getByTestId(`add-submit-${id}`).click();
};
const eq = (a, b) => JSON.stringify(a) === JSON.stringify(b);
const expectEq = (got, want, what) => { if (!eq(got, want)) throw new Error(`${what}: got ${JSON.stringify(got)}, want ${JSON.stringify(want)}`); };
const cardByTitle = (p, id, title) => col(p, id).getByTestId("card").filter({ has: p.getByTestId("card-title").getByText(title, { exact: true }) });

browser = await chromium.launch();

await check("three columns in order with headings, empty board", 4, async (p) => {
  const order = await p.evaluate(() => ["todo", "doing", "done"].map((id) => {
    const el = document.querySelector(`[data-testid="column-${id}"]`);
    return el ? [...document.querySelectorAll('[data-testid^="column-"]')].filter((x) => /^column-(todo|doing|done)$/.test(x.dataset.testid)).indexOf(el) : -1;
  }));
  expectEq(order, [0, 1, 2], "column DOM order");
  for (const [id, re] of [["todo", /to\s*do/i], ["doing", /doing/i], ["done", /done/i]]) {
    const heads = await col(p, id).locator("h1,h2,h3,h4,h5,h6,[role=heading]").allTextContents();
    if (!heads.some((h) => re.test(h))) throw new Error(`no heading for ${id}`);
  }
  for (const id of ["todo", "doing", "done"]) expectEq(await titles(p, id), [], `${id} cards at start`);
});

await check("add a card with the Add button", 6, async (p) => {
  await add(p, "todo", "Write tests");
  expectEq(await titles(p, "todo"), ["Write tests"], "todo titles");
  expectEq(await p.getByTestId("add-input-todo").inputValue(), "", "input cleared");
});

await check("add with Enter, title is trimmed", 4, async (p) => {
  await add(p, "doing", "  Spaced out  ", true);
  expectEq(await titles(p, "doing"), ["Spaced out"], "doing titles");
});

await check("empty and whitespace-only titles create nothing", 4, async (p) => {
  await add(p, "todo", "");
  await add(p, "todo", "    ");
  await add(p, "todo", "   ", true);
  expectEq(await titles(p, "todo"), [], "todo titles");
});

await check("column counts track the cards shown", 4, async (p) => {
  for (const t of ["a", "b", "c"]) await add(p, "todo", t);
  await add(p, "doing", "d");
  const txt = async (id) => (await p.getByTestId(`column-count-${id}`).textContent()).trim();
  expectEq([await txt("todo"), await txt("doing"), await txt("done")], ["3", "1", "0"], "counts");
  await p.getByTestId("card-delete").first().click();
  expectEq(await txt("todo"), "2", "todo count after delete");
});

await check("edit title with Edit then Enter, and with Save", 5, async (p) => {
  await add(p, "todo", "Old title");
  await p.getByTestId("card-edit").click();
  const input = p.getByTestId("card-edit-input");
  await input.waitFor();
  expectEq(await input.inputValue(), "Old title", "prefilled value");
  if (!(await input.evaluate((el) => el === document.activeElement))) throw new Error("edit input not focused");
  await input.fill("New title");
  await input.press("Enter");
  expectEq(await titles(p, "todo"), ["New title"], "after Enter");
  await p.getByTestId("card-edit").click();
  await p.getByTestId("card-edit-input").fill("Saved by button");
  await p.getByTestId("card-edit-save").click();
  expectEq(await titles(p, "todo"), ["Saved by button"], "after Save");
});

await check("edit: Escape cancels, empty title rejected", 3, async (p) => {
  await add(p, "todo", "Keep me");
  await p.getByTestId("card-edit").click();
  await p.getByTestId("card-edit-input").fill("changed");
  await p.getByTestId("card-edit-input").press("Escape");
  expectEq(await titles(p, "todo"), ["Keep me"], "after Escape");
  await p.getByTestId("card-edit").click();
  await p.getByTestId("card-edit-input").fill("   ");
  await p.getByTestId("card-edit-input").press("Enter");
  if (await p.getByTestId("card-edit-input").count()) await p.getByTestId("card-edit-input").press("Escape");
  expectEq(await titles(p, "todo"), ["Keep me"], "after empty save");
});

await check("delete removes the card immediately", 4, async (p) => {
  await add(p, "todo", "one");
  await add(p, "todo", "two");
  await cardByTitle(p, "todo", "one").getByTestId("card-delete").click();
  expectEq(await titles(p, "todo"), ["two"], "titles");
});

await check("move via the select goes to the end of the target column", 8, async (p) => {
  await add(p, "todo", "A");
  await add(p, "todo", "B");
  await add(p, "doing", "C");
  await cardByTitle(p, "todo", "A").getByTestId("card-move").selectOption("doing");
  expectEq([await titles(p, "todo"), await titles(p, "doing")], [["B"], ["C", "A"]], "after A->doing");
  await cardByTitle(p, "doing", "C").getByTestId("card-move").selectOption("done");
  await cardByTitle(p, "todo", "B").getByTestId("card-move").selectOption("doing");
  expectEq([await titles(p, "todo"), await titles(p, "doing"), await titles(p, "done")], [[], ["A", "B"], ["C"]], "final");
  const sel = await cardByTitle(p, "done", "C").getByTestId("card-move").inputValue();
  expectEq(sel, "done", "select reflects column");
});

await check("order within a column is stable", 4, async (p) => {
  for (const t of ["X", "Y", "Z"]) await add(p, "todo", t);
  await cardByTitle(p, "todo", "Y").getByTestId("card-delete").click();
  await add(p, "todo", "W");
  await cardByTitle(p, "todo", "X").getByTestId("card-edit").click();
  await p.getByTestId("card-edit-input").fill("X2");
  await p.getByTestId("card-edit-input").press("Enter");
  expectEq(await titles(p, "todo"), ["X2", "Z", "W"], "order");
});

await check("filter is case-insensitive, hides not deletes, counts follow", 7, async (p) => {
  for (const t of ["Apple pie", "apple tart", "Banana"]) await add(p, "todo", t);
  await add(p, "doing", "Cherry APPLE");
  await p.getByTestId("filter-input").fill("ApPlE");
  expectEq([await titles(p, "todo"), await titles(p, "doing")], [["Apple pie", "apple tart"], ["Cherry APPLE"]], "filtered");
  expectEq((await p.getByTestId("column-count-todo").textContent()).trim(), "2", "filtered count");
  await p.getByTestId("filter-input").fill("zzz-no-match");
  for (const id of ["todo", "doing", "done"]) expectEq(await titles(p, id), [], `${id} filtered out`);
  await p.getByTestId("filter-input").fill("");
  expectEq(await titles(p, "todo"), ["Apple pie", "apple tart", "Banana"], "restored");
});

await check("empty-state text per column", 3, async (p) => {
  for (const id of ["todo", "doing", "done"]) {
    const el = p.getByTestId(`column-empty-${id}`);
    await el.waitFor();
    if (!/no cards/i.test(await el.textContent())) throw new Error("text is not 'No cards'");
  }
  await add(p, "todo", "x");
  expectEq(await p.getByTestId("column-empty-todo").count(), 0, "empty-state gone in todo");
  expectEq(await p.getByTestId("column-empty-doing").count(), 1, "empty-state stays in doing");
});

await check("board persists across reload", 7, async (p) => {
  await add(p, "todo", "P1");
  await add(p, "todo", "P2");
  await add(p, "doing", "P3");
  await cardByTitle(p, "todo", "P1").getByTestId("card-move").selectOption("done");
  await p.reload();
  await p.getByTestId("board").waitFor();
  expectEq([await titles(p, "todo"), await titles(p, "doing"), await titles(p, "done")], [["P2"], ["P3"], ["P1"]], "after reload");
});

await check("drag and drop moves a card to another column", 6, async (p) => {
  await add(p, "todo", "Drag me");
  await add(p, "todo", "Stay");
  // Grab the title first (the card centre may sit on a button or select, which cannot start a drag), then the card itself.
  const drag = async (from, to, title) => {
    const c = cardByTitle(p, from, title);
    await c.getByTestId("card-title").dragTo(col(p, to));
    if (!(await titles(p, to)).includes(title)) await c.dragTo(col(p, to));
  };
  await drag("todo", "doing", "Drag me");
  expectEq([await titles(p, "todo"), await titles(p, "doing")], [["Stay"], ["Drag me"]], "after drag");
  await drag("doing", "done", "Drag me");
  expectEq(await titles(p, "done"), ["Drag me"], "second drag");
});

await check("unicode and emoji titles survive reload", 2, async (p) => {
  const t = "Café ☕ 你好 🚀";
  await add(p, "todo", t);
  await p.reload();
  await p.getByTestId("board").waitFor();
  expectEq(await titles(p, "todo"), [t], "title");
});

async function noHScroll(p) {
  return p.evaluate(() => ({ sw: document.documentElement.scrollWidth, cw: document.documentElement.clientWidth }));
}

{
  let t;
  try {
    t = await newPage({ width: 390, height: 800 });
    const p = t.page;
    await add(p, "todo", "x".repeat(300));
    await add(p, "todo", "A very long title with spaces ".repeat(8));
    const r = await noHScroll(p);
    const fits = await p.evaluate(() => [...document.querySelectorAll('[data-testid="card"]')].every((c) => c.getBoundingClientRect().right <= window.innerWidth + 1));
    const ok = r.sw <= r.cw + 1 && fits;
    checks.push({ name: "very long titles wrap (390px, no horizontal scroll)", weight: 3, score: ok ? 3 : 0, pass: ok, detail: ok ? "" : `scrollWidth ${r.sw} > ${r.cw} or card outside viewport` });
    for (const id of ["doing", "done"]) await add(p, id, "card in " + id);
    const r2 = await noHScroll(p);
    const ok2 = r2.sw <= r2.cw + 1;
    checks.push({ name: "mobile layout 390px: no horizontal scroll, board usable", weight: 3, score: ok2 ? 3 : 0, pass: ok2, detail: ok2 ? "" : `scrollWidth ${r2.sw} > ${r2.cw}` });
  } catch (e) {
    checks.push({ name: "very long titles wrap (390px, no horizontal scroll)", weight: 3, score: 0, pass: false, detail: String(e.message).split("\n")[0].slice(0, 200) });
    checks.push({ name: "mobile layout 390px: no horizontal scroll, board usable", weight: 3, score: 0, pass: false, detail: "see previous" });
  }
  if (t) await t.ctx.close();
}

await check("accessibility (axe): no serious/critical violations", 3, async (p) => {
  for (const id of ["todo", "doing", "done"]) await add(p, id, `Sample card in ${id}`);
  const res = await new AxeBuilder({ page: p }).analyze();
  const bad = res.violations.filter((v) => v.impact === "serious" || v.impact === "critical");
  const frac = bad.length === 0 ? 1 : bad.length <= 2 ? 0.5 : 0;
  return { frac, detail: bad.map((v) => `${v.id}(${v.nodes.length})`).join(", ") };
});

{
  const n = consoleErrors.length;
  checks.push({ name: "no console errors, page errors or dialogs", weight: 4, score: n === 0 ? 4 : 0, pass: n === 0, detail: n ? consoleErrors.slice(0, 3).join(" | ") : "" });
}

await browser.close();
console.log(JSON.stringify({ checks }));
