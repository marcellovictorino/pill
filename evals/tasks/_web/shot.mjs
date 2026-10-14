// usage: node shot.mjs <baseUrl> <json {outDir}>  -> writes 390.png and 1440.png (full page)
import { chromium } from "playwright";
import fs from "node:fs";

const url = process.argv[2];
const { outDir } = JSON.parse(process.argv[3]);
fs.mkdirSync(outDir, { recursive: true });
const browser = await chromium.launch();
for (const w of [390, 1440]) {
  const ctx = await browser.newContext({ viewport: { width: w, height: 900 } });
  const page = await ctx.newPage();
  await page.goto(url, { waitUntil: "load" });
  await page.evaluate(async () => {
    for (let y = 0; y < document.body.scrollHeight; y += 600) { window.scrollTo(0, y); await new Promise((r) => setTimeout(r, 40)); }
    window.scrollTo(0, 0);
  });
  await page.waitForTimeout(400);
  await page.screenshot({ path: `${outDir}/${w}.png`, fullPage: true });
  await ctx.close();
}
await browser.close();
console.log(JSON.stringify({ checks: [] }));
