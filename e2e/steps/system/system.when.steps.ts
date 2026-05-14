import { When } from '@cucumber/cucumber';
import { AudiodWorld } from '../../support/world';

When('User navigates to the application', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/');
});

When('User creates account with username {string} and password {string}', async function (this: AudiodWorld, username: string, password: string) {
	const page = this.getPage();

	await page.fill('[data-testid="username-input"]', username);
	await page.fill('[data-testid="password-input"]', password);
	await page.fill('[data-testid="confirm-password-input"]', password);
	await page.click('[data-testid="submit-init-button"]');

	this.currentUser = username;
});

// Same as the no-toggle variant above, but ticks the "Skip login on this
// device" checkbox before submit. The init page handles disabling auth in
// a follow-up request after the admin row lands; we just wait for the post-
// submit nav so subsequent steps see the toggled state.
When(
	'User creates account with username {string} and password {string} and disables login',
	async function (this: AudiodWorld, username: string, password: string) {
		const page = this.getPage();

		await page.fill('[data-testid="username-input"]', username);
		await page.fill('[data-testid="password-input"]', password);
		await page.fill('[data-testid="confirm-password-input"]', password);
		await page.check('[data-testid="disable-login-checkbox"]');
		// Wait for the PUT /api/settings/auth that fires after the setup
		// call returns — without it the next assertion races the toggle.
		const authPut = page.waitForResponse(
			(r) =>
				r.url().includes('/api/settings/auth') && r.request().method() === 'PUT',
			{ timeout: 5000 }
		);
		await page.click('[data-testid="submit-init-button"]');
		await authPut.catch(() => {
			// Surfaced by the assertion that follows.
		});
		await page.waitForURL((url) => !url.pathname.includes('/init'), { timeout: 5000 });

		this.currentUser = username;
	}
);

When('User attempts to access the initialization page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/init');
});

When('User visits any page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/');
});

// Weak/strong test fixtures — small but not empty (empty is the only hard reject),
// and well over 12 chars so the short-password warning does not fire.
const WEAK_PASSWORD = 'p';
const STRONG_PASSWORD = 'VeryStrongPassword2026';

When('User enters a weak password', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.fill('[data-testid="password-input"]', WEAK_PASSWORD);
});

When('User enters a strong password', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.fill('[data-testid="password-input"]', STRONG_PASSWORD);
});

When(
	'User enters username {string} and a password matching the username',
	async function (this: AudiodWorld, username: string) {
		const page = this.getPage();
		await page.fill('[data-testid="username-input"]', username);
		await page.fill('[data-testid="password-input"]', username);
	}
);

When(
	'User creates account with username {string} and a weak password',
	async function (this: AudiodWorld, username: string) {
		const page = this.getPage();
		await page.fill('[data-testid="username-input"]', username);
		await page.fill('[data-testid="password-input"]', WEAK_PASSWORD);
		await page.fill('[data-testid="confirm-password-input"]', WEAK_PASSWORD);
		await page.click('[data-testid="submit-init-button"]');

		this.currentUser = username;
		this.currentPassword = WEAK_PASSWORD;
	}
);
