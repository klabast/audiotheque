import { When } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';

When(
	'User authenticates with username {string} and password {string}',
	async function (this: AudiodWorld, username: string, password: string) {
		const page = this.getPage();

		if (!page.url().includes('/login')) {
			await page.goto('/login');
		}

		await page.fill('[data-testid="username-input"]', username);
		await page.fill('[data-testid="password-input"]', password);

		await page.click('[data-testid="submit-login-button"]');

		await Promise.race([
			page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 3000 }).catch(() => {}),
			page.waitForSelector('[data-testid="login-error"]', { timeout: 3000 }).catch(() => {})
		]);

		this.currentUser = username;
	}
);

// "...without keeping logged in" mirrors the default login flow today — there
// is no "Keep me logged in" checkbox in this slice. Once the checkbox lands
// (next slice), this step ensures it is unchecked before submission. The
// existing 30-day session expiry assertion is what differentiates this from
// the future "...and keeps logged in" variant (90-day window).
When(
	'User authenticates with username {string} and password {string} without keeping logged in',
	async function (this: AudiodWorld, username: string, password: string) {
		const page = this.getPage();

		if (!page.url().includes('/login')) {
			await page.goto('/login');
		}

		await page.fill('[data-testid="username-input"]', username);
		await page.fill('[data-testid="password-input"]', password);

		// "keep me logged in" checkbox may not exist yet; if present, ensure unchecked.
		const keep = page.locator('[data-testid="keep-logged-in-checkbox"]');
		if ((await keep.count()) > 0) {
			if (await keep.isChecked()) {
				await keep.uncheck();
			}
		}

		await page.click('[data-testid="submit-login-button"]');

		await Promise.race([
			page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 3000 }).catch(() => {}),
			page.waitForSelector('[data-testid="login-error"]', { timeout: 3000 }).catch(() => {})
		]);

		this.currentUser = username;
	}
);

When(
	'User changes username to {string} and password to {string}',
	async function (this: AudiodWorld, newUsername: string, newPassword: string) {
		const page = this.getPage();

		await page.fill('[data-testid="username-input"]', newUsername);
		await page.fill('[data-testid="password-input"]', newPassword);
		await page.fill('[data-testid="confirm-password-input"]', newPassword);

		await page.click('[data-testid="submit-init-button"]');
		await page.waitForLoadState('networkidle');

		this.currentUser = newUsername;
	}
);

When('User confirms the reset warning', async function (this: AudiodWorld) {
	// TODO: Add data-testid to reset account confirmation button
	await this.page!.click('button:has-text("I understand, reset my account")');
});

When('User logs out', async function (this: AudiodWorld) {
	const page = this.getPage();

	await page.locator('[data-testid="hamburger-menu-button"]').click();
	await page.locator('[data-testid="drawer-logout-button"]').click();

	await page.waitForLoadState('networkidle');
});

When('User requests password reset for {string}', async function (this: AudiodWorld, username: string) {
	const page = this.getPage();
	await page.goto('/forgot-password');
	await page.fill('[data-testid="username-input"]', username);
	await page.click('[data-testid="request-password-reset-button"]');
	await page.waitForLoadState('networkidle');
});

When('User enters valid reset code', async function (this: AudiodWorld) {
	const page = this.getPage();
	expect(this.resetCode).toBeDefined();
	await page.fill('[data-testid="reset-code-input"]', this.resetCode!);
});

When('User resets password to {string}', async function (this: AudiodWorld, newPassword: string) {
	const page = this.getPage();
	await page.fill('[data-testid="new-password-input"]', newPassword);
	await page.fill('[data-testid="confirm-password-input"]', newPassword);
	const confirmCall = page.waitForResponse(
		(r) => r.url().includes('/api/auth/password/reset/confirm'),
		{ timeout: 5000 }
	);
	await page.click('[data-testid="confirm-password-reset-button"]');
	const resp = await confirmCall.catch(() => null);
	if (!resp) {
		throw new Error('UI never called /api/auth/password/reset/confirm');
	}
	if (!resp.ok()) {
		throw new Error(
			`Confirm reset returned ${resp.status()}: ${await resp.text()}`
		);
	}
	await page.waitForLoadState('networkidle');
	this.currentPassword = newPassword;
});
