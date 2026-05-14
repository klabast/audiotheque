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

// "...without keeping logged in" explicitly ensures the "Keep me logged in"
// checkbox is unchecked before submitting the login form. Differentiates
// the 30-day default-window assertion from the 90-day remember-me variant
// (the "...and keeps logged in" step below).
When(
	'User authenticates with username {string} and password {string} without keeping logged in',
	async function (this: AudiodWorld, username: string, password: string) {
		const page = this.getPage();

		if (!page.url().includes('/login')) {
			await page.goto('/login');
		}

		await page.fill('[data-testid="username-input"]', username);
		await page.fill('[data-testid="password-input"]', password);

		const keep = page.locator('[data-testid="keep-logged-in-checkbox"]');
		if (await keep.isChecked()) {
			await keep.uncheck();
		}

		await page.click('[data-testid="submit-login-button"]');

		await Promise.race([
			page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 3000 }).catch(() => {}),
			page.waitForSelector('[data-testid="login-error"]', { timeout: 3000 }).catch(() => {})
		]);

		this.currentUser = username;
	}
);

// "...and keeps logged in" checks the persistent-session checkbox before
// submitting. Server then issues a 90-day cookie window instead of the
// 30-day default.
When(
	'User authenticates with username {string} and password {string} and keeps logged in',
	async function (this: AudiodWorld, username: string, password: string) {
		const page = this.getPage();

		if (!page.url().includes('/login')) {
			await page.goto('/login');
		}

		await page.fill('[data-testid="username-input"]', username);
		await page.fill('[data-testid="password-input"]', password);
		await page.check('[data-testid="keep-logged-in-checkbox"]');

		await page.click('[data-testid="submit-login-button"]');

		await Promise.race([
			page.waitForURL((url) => !url.pathname.includes('/login'), { timeout: 3000 }).catch(() => {}),
			page.waitForSelector('[data-testid="login-error"]', { timeout: 3000 }).catch(() => {})
		]);

		this.currentUser = username;
	}
);

// Navigates the page to "/" (the library home) and waits for the first
// authenticated API call to complete. That call is what triggers sliding-
// renewal server-side; without waiting for it the next assertion races
// the response that bumps expires_at and re-issues the cookie.
// "Open active devices" navigates the named browser to the Security tab of
// Settings, where the active-session rows are listed. Used by the Active
// Devices scenarios.
async function openSecuritySettings(page: import('@playwright/test').Page) {
	await page.goto('/settings/security');
	await page.waitForSelector('[data-testid="active-sessions-list"]', { timeout: 5000 });
}

When('User opens active devices in security settings', async function (this: AudiodWorld) {
	await openSecuritySettings(this.getPage());
});

When(
	'User on browser {string} opens active devices in security settings',
	async function (this: AudiodWorld, browser: string) {
		await openSecuritySettings(this.getBrowser(browser));
	}
);

// On browser X's Security tab, find the row that ISN'T the current session
// (i.e. belongs to some other browser) and click its ✕. The plan deliberately
// avoids passing the specific public id from one test step to another —
// "revoke the other session" is the user-visible semantic.
When(
	'User on browser {string} revokes the session on browser {string}',
	async function (this: AudiodWorld, fromBrowser: string, _targetBrowser: string) {
		const page = this.getBrowser(fromBrowser);
		if (!page.url().includes('/settings/security')) {
			await openSecuritySettings(page);
		}
		const rows = page.locator('[data-testid^="session-row-"]');
		const count = await rows.count();
		for (let i = 0; i < count; i++) {
			const row = rows.nth(i);
			const isCurrent =
				(await row.locator('[data-testid="current-session-badge"]').count()) > 0;
			if (!isCurrent) {
				await row.locator('[data-testid^="revoke-session-"]').click();
				await page.waitForResponse(
					(r) => r.url().includes('/api/auth/sessions/') && r.request().method() === 'DELETE',
					{ timeout: 5000 }
				);
				return;
			}
		}
		throw new Error('No non-current session row found to revoke');
	}
);

When(
	'User on browser {string} logs out of all other devices',
	async function (this: AudiodWorld, browser: string) {
		const page = this.getBrowser(browser);
		if (!page.url().includes('/settings/security')) {
			await openSecuritySettings(page);
		}
		const respPromise = page.waitForResponse(
			(r) => r.url().includes('/api/auth/sessions/revoke-others'),
			{ timeout: 5000 }
		);
		await page.click('[data-testid="logout-others-button"]');
		await respPromise;
	}
);

When(
	'User on browser {string} logs out of all devices',
	async function (this: AudiodWorld, browser: string) {
		const page = this.getBrowser(browser);
		if (!page.url().includes('/settings/security')) {
			await openSecuritySettings(page);
		}
		// "Log out of all devices" is sudo-gated: clicking the button opens
		// the password-confirm modal; the actual revoke fires only after
		// the modal accepts the password.
		await page.click('[data-testid="logout-all-button"]');
		await page.waitForSelector('[data-testid="logout-all-sudo-modal"]', { timeout: 5000 });
		const password = this.currentPassword || 'alicepass123';
		await page.fill('[data-testid="logout-all-sudo-password-input"]', password);
		const respPromise = page.waitForResponse(
			(r) => r.url().includes('/api/auth/sessions/revoke-all'),
			{ timeout: 5000 }
		);
		await page.click('[data-testid="logout-all-sudo-confirm-button"]');
		await respPromise;
		await page.waitForURL(/\/login/, { timeout: 5000 });
	}
);

// "Attempts to" semantic: triggers the sudo-gated flow but stops at the
// modal — used by the sudo-confirmation scenarios that need to inspect the
// prompt before deciding what password to type.
When('User attempts to log out of all devices', async function (this: AudiodWorld) {
	const page = this.getPage();
	if (!page.url().includes('/settings/security')) {
		await openSecuritySettings(page);
	}
	await page.click('[data-testid="logout-all-button"]');
	await page.waitForSelector('[data-testid="logout-all-sudo-modal"]', { timeout: 5000 });
});

// Fills the open sudo modal and clicks Confirm. Pairs with "attempts to..."
// above. Wraps either a 204 (modal closes, wrapped action fires) or a 401
// (modal stays open, error visible) — the calling scenario asserts which.
When(
	'User confirms with password {string}',
	async function (this: AudiodWorld, password: string) {
		const page = this.getPage();
		await page.fill('[data-testid="logout-all-sudo-password-input"]', password);
		const resp = page.waitForResponse(
			(r) => r.url().includes('/api/auth/verify-password'),
			{ timeout: 5000 }
		);
		await page.click('[data-testid="logout-all-sudo-confirm-button"]');
		await resp.catch(() => {
			// Network error surfaces as a Then-step failure rather than here.
		});
		// Let any subsequent redirect / DOM update settle before the next step.
		await page.waitForLoadState('networkidle').catch(() => {});
	}
);

When('User browses the library', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Wait for any authenticated /api/* call to complete — that's where the
	// session lookup happens and the renewal cookie gets re-issued.
	const apiCall = page.waitForResponse(
		(r) => r.url().includes('/api/') && !r.url().includes('/api/system'),
		{ timeout: 5000 }
	);
	await page.goto('/');
	await apiCall.catch(() => {
		// If no API call fires within timeout, the assertion that follows
		// will surface the real issue (cookie not refreshed).
	});
});

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

// --- Auth-disabled / re-enable flow ---
//
// "User navigates to the application" is owned by system/system.when.steps.ts —
// keep a single definition there (cucumber treats duplicate matchers as
// ambiguous).

When('User opens user management in settings', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/settings/users');
	await page.waitForLoadState('networkidle');
});

When('User attempts to disable authentication', async function (this: AudiodWorld) {
	const page = this.getPage();
	if (!page.url().includes('/settings/security')) {
		await page.goto('/settings/security');
		await page.waitForSelector('[data-testid="auth-toggle-section"]', { timeout: 5000 });
	}
	await page.click('[data-testid="disable-auth-button"]');
	await page.waitForSelector('[data-testid="disable-auth-sudo-modal"]', { timeout: 5000 });
});

When('User cancels the disable-login warning', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.click('[data-testid="disable-auth-sudo-cancel-button"]');
});

When(
	'User confirms the disable-login warning with password {string}',
	async function (this: AudiodWorld, password: string) {
		const page = this.getPage();
		await page.fill('[data-testid="disable-auth-sudo-password-input"]', password);
		const putAuth = page.waitForResponse(
			(r) =>
				r.url().includes('/api/settings/auth') && r.request().method() === 'PUT',
			{ timeout: 5000 }
		);
		await page.click('[data-testid="disable-auth-sudo-confirm-button"]');
		await putAuth.catch(() => {
			// Surfaced by the assertion that follows.
		});
		await page.waitForLoadState('networkidle').catch(() => {});
	}
);

When(
	'User re-enables authentication with password {string}',
	async function (this: AudiodWorld, password: string) {
		const page = this.getPage();
		if (!page.url().includes('/settings/security')) {
			await page.goto('/settings/security');
			await page.waitForSelector('[data-testid="auth-toggle-section"]', { timeout: 5000 });
		}
		await page.click('[data-testid="enable-auth-button"]');
		await page.waitForSelector('[data-testid="enable-auth-sudo-modal"]', { timeout: 5000 });
		await page.fill('[data-testid="enable-auth-sudo-password-input"]', password);
		const putAuth = page.waitForResponse(
			(r) =>
				r.url().includes('/api/settings/auth') && r.request().method() === 'PUT',
			{ timeout: 5000 }
		);
		await page.click('[data-testid="enable-auth-sudo-confirm-button"]');
		await putAuth.catch(() => {});
		// Re-enable hops to /login automatically — wait for that.
		await page.waitForURL(/\/login/, { timeout: 5000 }).catch(() => {});
	}
);

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
