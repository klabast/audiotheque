import { Given } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { runAudiodCli } from '../../support/audiod-cli';

/**
 * Creates a user via CLI command
 */
export function createUser(username: string, password: string, isAdmin: boolean = false): void {
	const adminFlag = isAdmin ? '--admin' : '';
	runAudiodCli(`user create --username ${username} --password ${password} ${adminFlag}`);
}

Given(
	'User is authenticated as {string} with default credentials',
	async function (this: AudiodWorld, username: string) {
		const page = this.getPage();
		await page.goto('/login');
		await page.fill('[data-testid="username-input"]', username);
		await page.fill('[data-testid="password-input"]', 'audiod');

		await page.click('[data-testid="submit-login-button"]');
		await page.waitForLoadState('networkidle');

		this.currentUser = username;
	}
);

Given(
	'Admin-User {string} exists with password {string}',
	async function (this: AudiodWorld, username: string, password: string) {
		createUser(username, password, true);
		this.currentUser = username;
		this.currentPassword = password;
		this.adminUser = username;
		this.adminPassword = password;
	}
);

Given(
	'User {string} exists with password {string}',
	async function (this: AudiodWorld, username: string, password: string) {
		createUser(username, password, false);
		this.currentUser = username;
		this.currentPassword = password;
	}
);

Given('User is on login page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/login');
	await expect(page).toHaveURL(/\/login/);
});

Given('User {string} is logged in', async function (this: AudiodWorld, username: string) {
	const page = this.getPage();
	const password = this.currentPassword || 'alicepass123';

	await page.context().clearCookies();

	await page.goto('/login');

	if (page.url().includes('/init')) {
		throw new Error('Cannot log in: System requires initialization. Run initialization scenarios first.');
	}

	await page.fill('[data-testid="username-input"]', username);
	await page.fill('[data-testid="password-input"]', password);
	await page.click('[data-testid="submit-login-button"]');
	await page.waitForURL((url) => !url.pathname.includes('/login'));

	expect(page.url()).not.toContain('/login');
	expect(page.url()).not.toContain('/init');

	this.currentUser = username;
});

Given('User is logged out', async function (this: AudiodWorld) {
	const page = this.getPage();

	await page.goto('/login');
	await expect(page).toHaveURL(/\/login/);
	this.currentUser = undefined;
	this.currentPassword = undefined;
});

// Plants a known-garbage session cookie before any navigation, simulating
// either a pre-rename JWT cookie hanging around in a browser or a session
// row that has been deleted server-side. The server should reject the
// cookie (no row matches) and the app should land on /login.
Given(
	'User has a stale session cookie for {string}',
	async function (this: AudiodWorld, username: string) {
		const page = this.getPage();
		// Cookies on `localhost` are visible to both :5180 (Vite) and :8880
		// (audiod) because cookies ignore port — dev mode runs both there,
		// CI mode runs everything on localhost too.
		await page.context().addCookies([
			{
				name: 'audiod_token',
				value: `stale-session-for-${username}-${Date.now()}`,
				domain: 'localhost',
				path: '/',
				httpOnly: true,
				secure: false,
				sameSite: 'Lax'
			}
		]);
		this.currentUser = undefined;
	}
);
