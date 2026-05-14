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

// The session cookie's Max-Age is set from the server's window (30d default,
// 90d remember-me). Playwright exposes `expires` as a unix timestamp;
// compare against now() with a generous tolerance so the assertion isn't
// fragile against minor clock skew or request latency.
Then(
	'Session is set to expire in approximately {int} days',
	async function (this: AudiodWorld, days: number) {
		const cookies = await this.getPage().context().cookies();
		const sess = cookies.find((c) => c.name === 'audiod_token');
		if (!sess) {
			throw new Error('audiod_token cookie not set after login');
		}
		const nowSec = Date.now() / 1000;
		const expectedSec = days * 86400;
		const actualSec = sess.expires - nowSec;
		// ±1 hour tolerance — generous enough for slow CI but tight enough to
		// catch a 7-day vs 30-day vs 90-day misconfiguration.
		const lower = expectedSec - 3600;
		const upper = expectedSec + 3600;
		if (actualSec < lower || actualSec > upper) {
			throw new Error(
				`Session cookie expires in ~${(actualSec / 86400).toFixed(2)}d; expected ~${days}d (got ${actualSec}s, window ${lower}..${upper}s)`
			);
		}
	}
);

// After "Session is past the halfway point of its window" + an authenticated
// request, the server should have bumped expires_at to ~30 days (default
// window) and re-issued the cookie. Any value clearly above the "~1 minute"
// state expire-soon left behind proves renewal happened — we assert at
// least 24 days remaining as a robust lower bound.
Then('Session expiry is renewed', async function (this: AudiodWorld) {
	const cookies = await this.getPage().context().cookies();
	const sess = cookies.find((c) => c.name === 'audiod_token');
	if (!sess) {
		throw new Error('audiod_token cookie missing after renewal step');
	}
	const remainingSec = sess.expires - Date.now() / 1000;
	const minSec = 24 * 86400;
	if (remainingSec < minSec) {
		throw new Error(
			`Session expects renewal: expires in ~${(remainingSec / 86400).toFixed(2)}d, expected ≥ 24d (sliding renewal did not fire)`
		);
	}
});

Then(
	'User sees {int} active session',
	async function (this: AudiodWorld, count: number) {
		const page = this.getPage();
		const rows = page.locator('[data-testid^="session-row-"]');
		await expect(rows).toHaveCount(count);
	}
);

Then(
	'User sees {int} active sessions',
	async function (this: AudiodWorld, count: number) {
		const page = this.getPage();
		const rows = page.locator('[data-testid^="session-row-"]');
		await expect(rows).toHaveCount(count);
	}
);

Then('Current session is marked as current', async function (this: AudiodWorld) {
	const page = this.getPage();
	const badges = page.locator('[data-testid="current-session-badge"]');
	// Exactly one row should bear the "current" badge regardless of how many
	// other sessions are listed.
	await expect(badges).toHaveCount(1);
});

// "Remains logged in" checks that the named browser, after some destructive
// action elsewhere, can still navigate to an authenticated page without
// being kicked to /login. Goes to / (the library home) and asserts the URL
// stays out of /login or /init.
Then(
	'Browser {string} remains logged in as {string}',
	async function (this: AudiodWorld, browser: string, username: string) {
		const page = this.getBrowser(browser);
		await page.goto('/');
		await expect(page).not.toHaveURL(/\/login/);
		await expect(page).not.toHaveURL(/\/init/);
		this.currentUser = username;
	}
);

Then(
	'Browser {string} is logged out',
	async function (this: AudiodWorld, browser: string) {
		const page = this.getBrowser(browser);
		await expect(page).toHaveURL(/\/login/);
	}
);

// "Logged out on next request" navigates the named browser to a protected
// page and asserts the server bounces it to /login because its session row
// was revoked from another browser.
Then(
	'Browser {string} is logged out on next request',
	async function (this: AudiodWorld, browser: string) {
		const page = this.getBrowser(browser);
		await page.goto('/');
		await page.waitForURL(/\/login/, { timeout: 5000 });
	}
);

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
