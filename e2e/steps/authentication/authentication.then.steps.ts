import { Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { getServerLogs, parseResetCode } from '../../support/server';
import { listServerDataDir, readServerDataFile } from '../../support/audiod-cli';

Then('User should be redirected to the password change page', async function (this: AudiodWorld) {
	expect(this.page!.url()).toContain('/change-password');
});

Then('User should be authenticated as {string}', async function (this: AudiodWorld, username: string) {
	expect(this.page!.url()).not.toContain('/login');
	this.currentUser = username;
});

Then('User should be logged in as {string}', async function (this: AudiodWorld, username: string) {
	const page = this.getPage();
	await page.waitForURL((url) => !url.pathname.includes('/login') && !url.pathname.includes('/init'));
	expect(page.url()).not.toContain('/login');
	expect(page.url()).not.toContain('/init');
	this.currentUser = username;
});

Then('User should be logged out', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.waitForURL(/\/login/);
	expect(page.url()).toContain('/login');
});

Then('User is on library page', async function (this: AudiodWorld) {
	const page = this.getPage();
	try {
		await page.waitForURL(
			(url) => url.pathname === '/' || url.pathname.includes('/library'),
			{ timeout: 10000 }
		);
	} catch (e) {
		// eslint-disable-next-line no-console
		console.log('  URL when waiting for library:', page.url());
		// eslint-disable-next-line no-console
		console.log('  body excerpt:', (await page.content()).substring(0, 500));
		throw e;
	}
	expect(page.url()).toMatch(/\/(library)?$/);
});

Then('User should see error {string}', async function (this: AudiodWorld, errorMessage: string) {
	const page = this.getPage();
	const errorElement = page.locator('[data-testid="login-error"], [data-testid="init-error"], [data-testid="password-change-error"]').first();

	await expect(errorElement).toBeVisible();
	await expect(errorElement).toContainText(errorMessage);
});

Then('User should see the library page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.waitForURL((url) => !url.pathname.includes('/login') && !url.pathname.includes('/change-password'));
	expect(page.url()).not.toContain('/login');
	expect(page.url()).not.toContain('/change-password');
});

Then('Server generates a reset code', async function (this: AudiodWorld) {
	// This happens on the server side - we'll verify it in the next step
});

Then('Reset code is logged to server console', async function (this: AudiodWorld) {
	const logs = await getServerLogs();
	this.resetCode = parseResetCode(logs);

	expect(this.resetCode).toBeDefined();
	expect(this.resetCode).toMatch(/^[A-Z0-9]+$/);
});

Then('Weak password warning is shown', async function (this: AudiodWorld) {
	const page = this.getPage();
	const warning = page.locator('[data-testid="weak-password-warning"]');
	await expect(warning).toBeVisible();
});

Then('Weak password warning is not shown', async function (this: AudiodWorld) {
	const page = this.getPage();
	const warning = page.locator('[data-testid="weak-password-warning"]');
	await expect(warning).toBeHidden();
});

Then(
	'Reset code is generated for {string}',
	{ timeout: 30000 },
	async function (this: AudiodWorld, username: string) {
		// File write happens synchronously when the request handler returns.
		// Allow a couple of polls in case the previous step's networkidle wait
		// raced ahead of the disk flush.
		let resetFiles: string[] = [];
		for (let i = 0; i < 10; i++) {
			const files = listServerDataDir('reset_codes');
			resetFiles = files
				.filter((f) => f.includes(`pw_reset_code_${username}`))
				.sort()
				.reverse();
			if (resetFiles.length > 0) break;
			await new Promise((r) => setTimeout(r, 200));
		}
		if (resetFiles.length === 0) {
			throw new Error(`No reset code file found for user ${username}`);
		}

		const fileContent = readServerDataFile(`reset_codes/${resetFiles[0]}`);
		const data = JSON.parse(fileContent);

		this.resetCode = data.code;
		expect(this.resetCode).toBeDefined();
		expect(this.resetCode).toMatch(/^[A-Z0-9]+$/);
	}
);
