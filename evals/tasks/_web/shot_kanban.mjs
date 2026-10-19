// usage: node shot_kanban.mjs <baseUrl> <json {outDir}> -> writes kanban.png: the board after adding a few cards through the UI.
// A broken Add button simply leaves the columns empty, which is what the screenshot should show.
import { chromium } from "playwright";
import fs from "node:fs";

const url = process.argv[2];
const { outDir } = JSON.parse(process.argv[3]);
fs.mkdirSync(outDir, { recursive: true });
const browser = await chromium.launch();
const page = await (await browser.newContext({ viewport: { width: 1440, height: 720 } })).newPage();
await page.goto(url, { waitUntil: "load" });
const cards = [["todo", "Write the project spec"], ["todo", "Review the pull request"], ["todo", "Plan the launch ✨"],
  ["doing", "Fix the login bug"], ["done", "Set up the repository"]];
for (const [id, text] of cards) {
  try {
    await page.getByTestId(`add-input-${id}`).fill(text, { timeout: 3000 });
    await page.getByTestId(`add-submit-${id}`).click({ timeout: 3000 });
  } catch { /* leave the board as it is */ }
}
await page.waitForTimeout(400);
await page.screenshot({ path: `${outDir}/kanban.png` });
await browser.close();
console.log(JSON.stringify({ checks: [] }));
