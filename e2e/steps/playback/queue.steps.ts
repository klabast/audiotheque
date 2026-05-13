import { Given, When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';

// Queue toggle in TopBar — present at all viewport sizes.
const TOGGLE = '[data-testid="toggle-queue-button"]';
const SIDEBAR = '[data-testid="right-sidebar"]';
const PANEL = '[data-testid="queue-panel"]';

async function isSidebarOpen(page: import('@playwright/test').Page): Promise<boolean> {
	// The aside is always in the DOM; "open" means its translate is 0 (the
	// `translate-x-0` class). The simplest reliable check: see whether the
	// QueuePanel content is rendered (we only render children when open).
	const panelCount = await page.locator(PANEL).count();
	if (panelCount === 0) return false;
	return await page.locator(PANEL).isVisible();
}

Given('Queue sidebar is open', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/');
	await page.waitForLoadState('domcontentloaded');
	if (!(await isSidebarOpen(page))) {
		await page.locator(TOGGLE).click();
		await page.locator(PANEL).waitFor({ state: 'visible', timeout: 3000 });
	}
});

When('User opens queue sidebar', async function (this: AudiodWorld) {
	const page = this.getPage();
	if (page.url() === 'about:blank' || !page.url().includes('/')) {
		await page.goto('/');
	}
	await page.waitForLoadState('domcontentloaded');
	await page.locator(TOGGLE).click();
	await page.locator(PANEL).waitFor({ state: 'visible', timeout: 3000 });
});

When('User closes queue sidebar', async function (this: AudiodWorld) {
	const page = this.getPage();
	// The close button has the same testid in both top-and-floating positions
	// of the panel — picking the first visible match is enough.
	const closeBtn = page.locator('[data-testid="close-queue-button"]').first();
	await closeBtn.click();
});

Then('Queue sidebar is visible', async function (this: AudiodWorld) {
	const page = this.getPage();
	await expect(page.locator(PANEL)).toBeVisible();
	await expect(page.locator(SIDEBAR)).toBeVisible();
});

Then('Queue sidebar is hidden', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Panel is only rendered when the sidebar is open, so visibility check works
	// for "closed" too. Wait for it to become hidden after close animation.
	await expect(page.locator(PANEL)).toBeHidden({ timeout: 1000 });
});

Then('Queue sidebar lists upcoming tracks', async function (this: AudiodWorld) {
	const page = this.getPage();
	await expect(page.locator(PANEL)).toBeVisible();

	// Either the explicit-queue list or the source list (next from album)
	// must show at least one entry. The QueuePanel fetches album tracks
	// asynchronously after `session.source` arrives, so we poll until one
	// of the lists renders.
	const explicit = page.locator('[data-testid="queue-explicit"]');
	const source = page.locator('[data-testid="queue-source"]');
	await expect
		.poll(
			async () => {
				const e = (await explicit.count()) > 0 && (await explicit.isVisible());
				const s = (await source.count()) > 0 && (await source.isVisible());
				return e || s;
			},
			{ timeout: 5000, message: 'expected explicit-queue or source list to render' }
		)
		.toBe(true);
});
