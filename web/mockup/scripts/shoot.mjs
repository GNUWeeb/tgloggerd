// Screenshot mockup pages in light and dark, at desktop and phone width,
// and report page errors and horizontal overflow.
//
// Usage: node scripts/shoot.mjs <outdir> <path>...
// Env:   MOCKUP_URL  dev server (default http://127.0.0.1:51737)
//        TGD_AS      sample account to view as (1, 3 or 5; see viewer.jsx)
import { chromium } from 'playwright';

const BASE = process.env.MOCKUP_URL || 'http://127.0.0.1:51737';
const [out, ...paths] = process.argv.slice(2);
const browser = await chromium.launch();
const errors = [];
for (const [vw, vh, tag] of [[1440, 900, 'desk'], [390, 844, 'phone']]) {
	for (const scheme of ['light', 'dark']) {
		const ctx = await browser.newContext({ viewport: { width: vw, height: vh }, colorScheme: scheme });
		if (process.env.TGD_AS) await ctx.addInitScript((as) => localStorage.setItem('tgd-as', as), process.env.TGD_AS);
		const page = await ctx.newPage();
		page.on('pageerror', (e) => errors.push(`${tag}/${scheme} ${page.url()}: ${e.message}`));
		page.on('console', (m) => m.type() === 'error' && errors.push(`${tag}/${scheme} console: ${m.text()}`));
		for (const p of paths) {
			await page.goto(BASE + p, { waitUntil: 'networkidle' });
			const name = p.replace(/[^a-z0-9]+/gi, '_').replace(/^_|_$/g, '') || 'root';
			await page.screenshot({ path: `${out}/${name}.${tag}.${scheme}.png` });
			const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
			if (overflow > 0) errors.push(`${tag}/${scheme} ${p}: horizontal overflow ${overflow}px`);
		}
		await ctx.close();
	}
}
await browser.close();
console.log(errors.length ? errors.join('\n') : 'no errors');
