import { When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';

// User-management steps. The admin Users tab UI drives all of these — no
// CLI shortcuts, since the scenarios are explicitly about the admin
// experience. Authenticated-as-admin is enforced by the Background of the
// feature; these steps assume the cookie is already set.

async function gotoUsersTab(page: import('@playwright/test').Page): Promise<void> {
	if (!page.url().includes('/settings/users')) {
		await page.goto('/settings/users');
	}
	await page.waitForSelector('[data-testid="create-user-button"]', { timeout: 5000 });
}

When(
	'User creates new user {string} with password {string}',
	async function (this: AudiodWorld, username: string, password: string) {
		const page = this.getPage();
		await gotoUsersTab(page);
		await page.fill('[data-testid="new-user-username-input"]', username);
		await page.fill('[data-testid="new-user-password-input"]', password);
		const post = page.waitForResponse(
			(r) => r.url().includes('/api/users') && r.request().method() === 'POST',
			{ timeout: 5000 }
		);
		await page.click('[data-testid="create-user-button"]');
		const resp = await post.catch(() => null);
		if (!resp || !resp.ok()) {
			throw new Error(
				`POST /api/users failed: ${resp?.status() ?? 'no response'} ${
					resp ? await resp.text() : ''
				}`
			);
		}
		// Wait for the new row to render so any subsequent action targets it.
		await page.waitForSelector(`[data-testid="user-row-${username}"]`, {
			timeout: 5000
		});
	}
);

When(
	'User deletes user {string}',
	async function (this: AudiodWorld, username: string) {
		const page = this.getPage();
		await gotoUsersTab(page);
		await page.click(`[data-testid="delete-user-${username}"]`);
		// ConfirmDeleteModal requires typing the username to enable the
		// confirm button.
		await page.waitForSelector('[data-testid="delete-user-confirm-modal"]', {
			timeout: 5000
		});
		await page.fill(
			'[data-testid="delete-user-confirm-confirmation-input"]',
			username
		);
		const del = page.waitForResponse(
			(r) =>
				r.url().includes(`/api/users/`) && r.request().method() === 'DELETE',
			{ timeout: 5000 }
		);
		await page.click('[data-testid="delete-user-confirm-confirm-button"]');
		const resp = await del.catch(() => null);
		if (!resp || !resp.ok()) {
			throw new Error(
				`DELETE /api/users failed: ${resp?.status() ?? 'no response'} ${
					resp ? await resp.text() : ''
				}`
			);
		}
		// Row should disappear once the refresh re-fetches the list.
		await page.waitForSelector(`[data-testid="user-row-${username}"]`, {
			state: 'detached',
			timeout: 5000
		});
	}
);

When(
	'User resets password for {string} to {string}',
	async function (this: AudiodWorld, username: string, newPassword: string) {
		const page = this.getPage();
		await gotoUsersTab(page);
		await page.click(`[data-testid="reset-password-${username}"]`);
		await page.waitForSelector('[data-testid="reset-password-modal"]', {
			timeout: 5000
		});
		await page.fill('[data-testid="reset-password-input"]', newPassword);
		const put = page.waitForResponse(
			(r) =>
				r.url().includes('/password') &&
				r.url().includes('/api/users/') &&
				r.request().method() === 'PUT',
			{ timeout: 5000 }
		);
		await page.click('[data-testid="reset-password-confirm-button"]');
		const resp = await put.catch(() => null);
		if (!resp || !resp.ok()) {
			throw new Error(
				`PUT /api/users/{id}/password failed: ${
					resp?.status() ?? 'no response'
				} ${resp ? await resp.text() : ''}`
			);
		}
	}
);

// Credential check via /api/auth/login — uses a fresh request context so the
// admin's cookie isn't reused. 200 → can authenticate; 401 → cannot.
async function tryLogin(
	world: AudiodWorld,
	username: string,
	password: string
): Promise<number> {
	const ctx = await world.getPage().context().browser()!.newContext();
	try {
		const resp = await ctx.request.post('http://localhost:8880/api/auth/login', {
			headers: { 'Content-Type': 'application/json' },
			data: { username, password }
		});
		return resp.status();
	} finally {
		await ctx.close();
	}
}

Then(
	'User {string} can authenticate with password {string}',
	async function (this: AudiodWorld, username: string, password: string) {
		const status = await tryLogin(this, username, password);
		expect(status, `login as ${username} expected 200, got ${status}`).toBe(200);
	}
);

Then(
	'User {string} cannot authenticate with password {string}',
	async function (this: AudiodWorld, username: string, password: string) {
		const status = await tryLogin(this, username, password);
		expect(
			status,
			`login as ${username} expected 401, got ${status}`
		).not.toBe(200);
	}
);
