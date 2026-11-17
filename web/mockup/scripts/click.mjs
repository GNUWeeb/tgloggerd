// Open a page, click through buttons or tabs by their accessible name,
// and screenshot the result (dialogs, menus, tabs).
//
// Usage: node scripts/click.mjs <out.png> <path> <'Name>>Name...'> [width] [light|dark]
// Env:   MOCKUP_URL, TGD_AS as for shoot.mjs
import { chromium } from 'playwright';

const BASE = process.env.MOCKUP_URL || 'http://127.0.0.1:51737';
const [out, path, names, w = '1440', scheme = 'light'] = process.argv.slice(2);
const browser = await chromium.launch();
const page = await browser.newPage({ viewport: { width: Number(w), height: 900 }, colorScheme: scheme });
if (process.env.TGD_AS) await page.addInitScript((as) => localStorage.setItem('tgd-as', as), process.env.TGD_AS);
const errors = [];
page.on('pageerror', (e) => errors.push(e.message));
await page.goto(BASE + path, { waitUntil: 'networkidle' });
for (const name of names.split('>>')) {
	const tries = [
		page.getByRole('button', { name, exact: true }),
		page.getByRole('tab', { name, exact: true }),
		page.getByRole('button', { name }),
	];
	let hit = tries[2];
	for (const t of tries) if (await t.count()) { hit = t; break; }
	await hit.first().click();
	await page.waitForTimeout(150);
}
await page.waitForTimeout(300);
await page.screenshot({ path: out });
await browser.close();
console.log(errors.length ? errors.join('\n') : 'no errors');
