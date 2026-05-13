import { Given } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { runAudiodCli, resolveServerPath } from '../../support/audiod-cli';

Given('No libraries exist', async function (this: AudiodWorld) {
	// No action needed - "System is in initial state" already resets everything
});

Given(
	'Library {string} exists with path(s) {string}',
	async function (this: AudiodWorld, libraryName: string, pathsString: string) {
		if (!this.currentUser || !this.currentPassword) {
			throw new Error('No user logged in. Use "User X is logged in" step before creating a library.');
		}

		const username = this.currentUser;
		const password = this.currentPassword;

		const paths = pathsString.split(',').map((p) => p.trim());
		const pathFlags = paths.map((p) => `--path "${resolveServerPath(p)}"`).join(' ');

		const output = runAudiodCli(
			`library create --name "${libraryName}" ${pathFlags} --user "${username}" --password "${password}"`
		);

		const idMatch = output.match(/ID:\s*(\d+)/);
		if (idMatch) {
			this.createdLibraryId = parseInt(idMatch[1], 10);
		}

		this.createdLibraryName = libraryName;
	}
);

Given(
	'User {string} has read access to library {string}',
	async function (this: AudiodWorld, targetUsername: string, _libraryName: string) {
		if (!this.adminUser || !this.adminPassword) {
			throw new Error('No admin user created. Use "Admin-User" step in Background first.');
		}
		if (!this.createdLibraryId) {
			throw new Error('No library ID stored. Create the library first.');
		}

		runAudiodCli(
			`library access grant --library ${this.createdLibraryId} --target-user "${targetUsername}" --user "${this.adminUser}" --password "${this.adminPassword}"`
		);
	}
);

Given(
	'Library {string} scan is complete',
	{ timeout: 90000 },
	async function (this: AudiodWorld, libraryName: string) {
		const page = this.getPage();

		const listResponse = await page.request.get('/api/libraries');
		expect(listResponse.ok()).toBe(true);
		const libraries = await listResponse.json();
		const library = libraries.find((l: { name: string }) => l.name === libraryName);
		if (!library) {
			throw new Error(`Library "${libraryName}" not found`);
		}
		const libraryId = library.id;

		// Poll the authoritative scan-status endpoint: 204 = no pending/running
		// scan job for this library. The UI's progress bar reflects SSE events
		// and lags after a page reload, which produced false-positive "complete"
		// reads while the scanner was still writing.
		await expect(async () => {
			const response = await page.request.get(`/api/libraries/${libraryId}/scan-status`);
			if (response.status() !== 204) {
				throw new Error(`scan still in progress (status ${response.status()})`);
			}
		}).toPass({ timeout: 60000, intervals: [200, 500, 1000] });
	}
);

Given('User is on library browse page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/');
	await page.waitForLoadState('networkidle');

	const albumGrid = page.locator('[data-testid="album-grid"]');
	await expect(albumGrid).toBeVisible({ timeout: 10000 });
});

Given('User is on album details page', async function (this: AudiodWorld) {
	const page = this.getPage();

	await page.goto('/');
	await page.waitForLoadState('networkidle');

	const albumGrid = page.locator('[data-testid="album-grid"]');
	await expect(albumGrid).toBeVisible({ timeout: 10000 });

	const firstAlbum = page.locator('[data-testid^="album-card-"]').first();
	await firstAlbum.click();

	await page.waitForURL('**/album/**');
});

Given('User has scrolled down in the album grid', async function (this: AudiodWorld) {
	const page = this.getPage();

	const scrollTop = await page.evaluate(() => {
		const main = document.querySelector('main');
		if (!main) return 0;
		main.scrollTop = main.scrollHeight;
		return main.scrollTop;
	});

	if (scrollTop === 0) {
		throw new Error(
			'Album grid did not scroll — fixture has too few albums for this viewport. Add more test albums or use a smaller viewport.'
		);
	}

	this.savedScrollTop = scrollTop;
});

Given('Library shows both hi-res and standard albums', async function (this: AudiodWorld) {
	const page = this.getPage();

	const total = await page.locator('[data-testid^="album-card-"]').count();
	expect(total).toBeGreaterThan(1);

	const hiResCount = await page.locator('[data-testid^="album-card-"][data-hires="true"]').count();
	const standardCount = await page
		.locator('[data-testid^="album-card-"][data-hires="false"]')
		.count();
	expect(hiResCount).toBeGreaterThan(0);
	expect(standardCount).toBeGreaterThan(0);

	this.initialAlbumCount = total;
	this.initialHiResAlbumCount = hiResCount;
});
