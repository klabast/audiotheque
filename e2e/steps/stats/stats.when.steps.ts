/**
 * Stats steps - When
 */
import { When } from '@cucumber/cucumber';
import { AudiodWorld } from '../../support/world';

When('User looks at stats for this week', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.locator('[data-testid="hamburger-menu-button"]').click();
	await page.locator('[data-testid="nav-stats-link"]').click();
	const statsLoaded = page.waitForResponse(
		(r) => r.url().includes('/api/stats?period=week') && r.status() === 200
	);
	await page.locator('[data-testid="stats-period-week-button"]').click();
	await statsLoaded;
});
