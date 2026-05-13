import { Given } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { runAudiodCli } from '../../support/audiod-cli';

Given('System is in initial state', async function (this: AudiodWorld) {
	runAudiodCli('system reset --confirm');
});

Given('User is on the initialization page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/init');
	await page.waitForURL('**/init');
});

Given('Admin user exists', async function (this: AudiodWorld) {
	const page = this.getPage();

	const response = await page.request.get('http://localhost:8880/api/system');
	const data = await response.json();

	expect(data.requiresAdminUser).toBe(false);
});
