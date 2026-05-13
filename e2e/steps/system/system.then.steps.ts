import { Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';

Then('User should see the initialization page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.waitForURL('**/init');
	expect(page.url()).toContain('/init');
});

Then('User should see a form to create the first account', async function (this: AudiodWorld) {
	const page = this.getPage();

	await expect(page.locator('[data-testid="username-input"]')).toBeVisible();
	await expect(page.locator('[data-testid="password-input"]')).toBeVisible();
	await expect(page.locator('[data-testid="submit-init-button"]')).toBeVisible();
});

Then('User should see the login page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.waitForURL('**/login');
	expect(page.url()).toContain('/login');
});

Then('User should see message {string}', async function (this: AudiodWorld, message: string) {
	const page = this.getPage();
	// TODO: Add data-testid for error messages
	await expect(page.locator(`text=${message}`)).toBeVisible();
});
