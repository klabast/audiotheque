import { When } from '@cucumber/cucumber';
import { AudiodWorld } from '../../support/world';

When(
	'User changes password from {string} to {string}',
	async function (this: AudiodWorld, oldPassword: string, newPassword: string) {
		const page = this.getPage();
		await page.goto('/settings/account');

		await page.fill('[data-testid="current-password-input"]', oldPassword);
		await page.fill('[data-testid="new-password-input"]', newPassword);
		await page.fill('[data-testid="confirm-password-input"]', newPassword);

		await page.click('[data-testid="change-password-button"]');
		await page.waitForLoadState('networkidle');

		this.currentPassword = newPassword;
	}
);

When(
	'User attempts to change password with mismatched confirmation',
	async function (this: AudiodWorld) {
		const page = this.getPage();

		await page.goto('/settings/account');

		await page.fill('[data-testid="current-password-input"]', this.currentPassword || 'alicepass123');
		await page.fill('[data-testid="new-password-input"]', 'newpass456');
		await page.fill('[data-testid="confirm-password-input"]', 'different123');

		await page.click('[data-testid="change-password-button"]');
		await page.waitForLoadState('networkidle');
	}
);

When('User attempts to change password to {string}', async function (this: AudiodWorld, newPassword: string) {
	const page = this.getPage();

	await page.goto('/settings/account');

	await page.fill('[data-testid="current-password-input"]', this.currentPassword || 'alicepass123');
	await page.fill('[data-testid="new-password-input"]', newPassword);
	await page.fill('[data-testid="confirm-password-input"]', newPassword);

	await page.click('[data-testid="change-password-button"]');
	await page.waitForLoadState('networkidle');
});

When(
	'User attempts to change password with wrong current password',
	async function (this: AudiodWorld) {
		const page = this.getPage();

		await page.goto('/settings/account');

		await page.fill('[data-testid="current-password-input"]', 'wrongpassword123');
		await page.fill('[data-testid="new-password-input"]', 'newpass456');
		await page.fill('[data-testid="confirm-password-input"]', 'newpass456');

		await page.click('[data-testid="change-password-button"]');
		await page.waitForLoadState('networkidle');
	}
);

When('User changes theme to {string}', async function (this: AudiodWorld, theme: string) {
	const page = this.getPage();

	await page.goto('/settings/general');

	await page.selectOption('[data-testid="theme-select"]', theme);

	await page.click('[data-testid="save-settings-button"]');
	await page.waitForLoadState('networkidle');
});

When('User changes language to {string}', async function (this: AudiodWorld, language: string) {
	const page = this.getPage();

	await page.goto('/settings/general');

	await page.selectOption('[data-testid="language-select"]', language);

	await page.click('[data-testid="save-settings-button"]');
	await page.waitForLoadState('networkidle');
});

// Fill the change-password form but do NOT click submit. Used by weak-password
// scenarios that need to assert the live warning is visible before submission.
When(
	'User starts changing password from {string} to {string}',
	async function (this: AudiodWorld, oldPassword: string, newPassword: string) {
		const page = this.getPage();
		await page.goto('/settings/account');

		await page.fill('[data-testid="current-password-input"]', oldPassword);
		await page.fill('[data-testid="new-password-input"]', newPassword);
		await page.fill('[data-testid="confirm-password-input"]', newPassword);

		// Stash so a follow-up "User logs out / authenticates" chain knows the new password.
		this.currentPassword = newPassword;
	}
);

When('User submits the password change', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.click('[data-testid="change-password-button"]');
	await page.waitForLoadState('networkidle');
});
