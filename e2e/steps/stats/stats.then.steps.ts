/**
 * Stats steps - Then
 */
import { Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';

Then(
	'Top track is {string} with {int} plays',
	async function (this: AudiodWorld, title: string, plays: number) {
		const top = this.getPage().locator('[data-testid="stats-top-tracks"] li').first();
		await expect(top).toContainText(title);
		await expect(top.locator('[data-testid="stats-top-track-plays"]')).toHaveText(String(plays));
	}
);

Then('Stats count {int} listens', async function (this: AudiodWorld, listens: number) {
	await expect(this.getPage().locator('[data-testid="stats-listens"]')).toHaveText(String(listens));
});

Then('Stats count no listens', async function (this: AudiodWorld) {
	await expect(this.getPage().locator('[data-testid="stats-empty"]')).toBeVisible();
});
