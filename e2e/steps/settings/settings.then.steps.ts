import { Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';

Then('Password change succeeds', async function (this: AudiodWorld) {
	const page = this.getPage();

	const successMessage = page.locator('[data-testid="password-change-success"]');
	await expect(successMessage).toBeVisible({ timeout: 5000 });
});

Then('App displays in dark theme', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/settings/general');
	const themeSelect = page.locator('[data-testid="theme-select"]');
	await expect(themeSelect).toHaveValue('dark');
});

Then('App displays in light theme', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/settings/general');
	const themeSelect = page.locator('[data-testid="theme-select"]');
	await expect(themeSelect).toHaveValue('light');
});

Then('App theme reflects system theme', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/settings/general');
	const themeSelect = page.locator('[data-testid="theme-select"]');
	await expect(themeSelect).toHaveValue('system');
});

Then('App displays in German language', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/settings/general');

	const languageSelect = page.locator('[data-testid="language-select"]');
	await expect(languageSelect).toHaveValue('de');

	await expect(page.getByText('Allgemeine Einstellungen')).toBeVisible();
});

Then('App displays in English language', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/settings/general');

	const languageSelect = page.locator('[data-testid="language-select"]');
	await expect(languageSelect).toHaveValue('en');

	await expect(page.getByText('General Settings')).toBeVisible();
});
