import { Given } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { runAudiodCli } from '../../support/audiod-cli';
import {
	getActiveDeviceConfig,
	getBaseURL,
	getSharedBrowser
} from '../../support/hooks';
import type { Page } from '@playwright/test';

// Helper: log a browser tab in as username/password by going through the
// real /login form. For browser "A" we reuse the primary page; any other
// name spins up a fresh BrowserContext via openFreshBrowser so the server
// sees a distinct session row (no storageState inheritance).
async function loginOnBrowser(
	world: AudiodWorld,
	name: string,
	username: string,
	password: string
): Promise<Page> {
	let page: Page;
	if (name === 'A' || name === 'a' || name === 'main') {
		page = world.getPage();
	} else {
		const existing = world.extraPages.get(name);
		if (existing) {
			page = existing;
		} else {
			page = await world.openFreshBrowser(
				name,
				getSharedBrowser(),
				getActiveDeviceConfig(),
				getBaseURL()
			);
		}
	}
	await page.context().clearCookies();
	await page.goto('/login');
	await page.fill('[data-testid="username-input"]', username);
	await page.fill('[data-testid="password-input"]', password);
	await page.click('[data-testid="submit-login-button"]');
	await page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 5000 });
	world.currentUser = username;
	return page;
}

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
		this.userPasswords.set(username, password);
	}
);

Given(
	'User {string} exists with password {string}',
	async function (this: AudiodWorld, username: string, password: string) {
		createUser(username, password, false);
		this.currentUser = username;
		this.currentPassword = password;
		this.userPasswords.set(username, password);
	}
);

Given('User is on login page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/login');
	await expect(page).toHaveURL(/\/login/);
});

Given('User {string} is logged in', async function (this: AudiodWorld, username: string) {
	const page = this.getPage();
	// Prefer the per-user password map so scenarios that switch accounts
	// resolve the right credential by name; fall back to currentPassword
	// (the original single-user behaviour) and finally a hardcoded default.
	const password =
		this.userPasswords.get(username) || this.currentPassword || 'alicepass123';

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
	this.currentPassword = password;
});

// Forces the current user's sessions to expire in ~1 minute via the
// `audiod session expire-soon` CLI fixture. Putting the session in the
// "less than half the window remains" zone guarantees the next
// authenticated request fires sliding-renewal in Service.ValidateSession.
Given('Session is past the halfway point of its window', async function (this: AudiodWorld) {
	const username = this.currentUser;
	if (!username) {
		throw new Error('No current user set — log in before invoking this step');
	}
	runAudiodCli(`session expire-soon --username ${username}`);
});

// Multi-device step: log a named browser tab in independently. Each call
// either reuses the named tab if it exists or opens a fresh context with
// no inherited auth, so the server records a distinct session row per
// browser. Used by the Active Devices scenarios.
Given(
	'User {string} is logged in on browser {string}',
	async function (this: AudiodWorld, username: string, browser: string) {
		await loginOnBrowser(this, browser, username, this.currentPassword || 'alicepass123');
	}
);

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

// Drives the auth-enabled toggle directly via the settings API. The step is
// the canonical Given/Then per CLAUDE.md guidance: cucumber matches one
// definition per phrase regardless of keyword, so a single function ensures
// the state (idempotent set-if-needed) AND asserts it at the end. Used as
// Given it sets up; used as Then it short-circuits if the previous step
// already produced the state, but still verifies — catching regressions
// where the prior When silently no-op'd.
async function ensureAuthEnabled(page: Page, want: boolean): Promise<void> {
	const get1 = await page.request.get('/api/settings/auth');
	if (!get1.ok()) {
		throw new Error(`GET /api/settings/auth failed: ${get1.status()}`);
	}
	const current = (await get1.json()) as { enabled: boolean };
	if (current.enabled !== want) {
		const put = await page.request.put('/api/settings/auth', {
			headers: { 'Content-Type': 'application/json' },
			data: { enabled: want }
		});
		if (!put.ok()) {
			throw new Error(
				`PUT /api/settings/auth failed: ${put.status()} ${await put.text()}`
			);
		}
	}
	// Re-read to confirm the state took. This is what makes the step useful
	// as a Then assertion — a prior When that should have flipped the toggle
	// but didn't will surface here even though the set-if-needed branch
	// would otherwise paper over it.
	const get2 = await page.request.get('/api/settings/auth');
	if (!get2.ok()) {
		throw new Error(`GET /api/settings/auth failed (verify): ${get2.status()}`);
	}
	const after = (await get2.json()) as { enabled: boolean };
	expect(after.enabled).toBe(want);
}

Given('Authentication is disabled', async function (this: AudiodWorld) {
	await ensureAuthEnabled(this.getPage(), false);
});

Given('Authentication is enabled', async function (this: AudiodWorld) {
	await ensureAuthEnabled(this.getPage(), true);
});
