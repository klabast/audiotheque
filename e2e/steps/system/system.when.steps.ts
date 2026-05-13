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

When('User attempts to access the initialization page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/init');
});

When('User visits any page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/');
});
