import { Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { resolveServerPath } from '../../support/audiod-cli';

Then('No-library message is visible', async function (this: AudiodWorld) {
	const page = this.getPage();
	const noLibraryMessage = page.locator('[data-testid="no-library-message"]');
	await expect(noLibraryMessage).toBeVisible();
});

Then('Library settings link is visible', async function (this: AudiodWorld) {
	const page = this.getPage();
	const settingsLink = page.locator('[data-testid="library-settings-link"]');
	await expect(settingsLink).toBeVisible();
});

Then('Library settings link is not visible', async function (this: AudiodWorld) {
	const page = this.getPage();
	const settingsLink = page.locator('[data-testid="library-settings-link"]');
	await expect(settingsLink).not.toBeVisible();
});

Then('User is on library settings page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.waitForURL('**/settings/library');
	expect(page.url()).toContain('/settings/library');
});

Then('Library settings page is read-only', async function (this: AudiodWorld) {
	const page = this.getPage();
	const createButton = page.locator('[data-testid="create-library-button"]');
	await expect(createButton).toBeVisible();
	await expect(createButton).toBeDisabled();
});

Then('Library {string} exists', async function (this: AudiodWorld, libraryName: string) {
	const page = this.getPage();
	const libraryListItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
	await expect(libraryListItem).toBeVisible();
});

Then('Library creation is rejected', async function (this: AudiodWorld) {
	const page = this.getPage();
	const validationError = page.locator('[data-testid="validation-error"]');
	await expect(validationError).toBeVisible();
});

Then('Library scanning completes successfully', async function (this: AudiodWorld) {
	const page = this.getPage();
	const scanCompleteIndicator = page.locator('[data-testid="library-scan-complete"]');
	await expect(scanCompleteIndicator).toBeVisible({ timeout: 30000 });
});

Then('Library is browsable', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/');
	await page.waitForLoadState('networkidle');

	const noLibraryMessage = page.locator('[data-testid="no-library-message"]');
	await expect(noLibraryMessage).not.toBeVisible();
});

Then('Library {string} does not exist', async function (this: AudiodWorld, libraryName: string) {
	const page = this.getPage();

	if (!page.url().includes('/settings/library')) {
		await page.goto('/settings/library');
		await page.waitForLoadState('networkidle');
	}

	const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
	await expect(libraryItem).not.toBeVisible();
});

Then(
	'Library list does not contain {string}',
	async function (this: AudiodWorld, libraryName: string) {
		const page = this.getPage();

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).not.toBeVisible();
	}
);

Then(
	'Delete library button is not visible for {string}',
	async function (this: AudiodWorld, _libraryName: string) {
		const page = this.getPage();

		const deleteButton = page.locator('[data-testid^="delete-library-button-"]');
		await expect(deleteButton).not.toBeVisible();
	}
);

Then('Saving the library is not possible', async function (this: AudiodWorld) {
	const page = this.getPage();
	const saveButton = page.locator('[data-testid="edit-library-save-button"]');
	await expect(saveButton).toBeDisabled();
});

Then(
	'Library {string} has paths {string}',
	async function (this: AudiodWorld, libraryName: string, pathsString: string) {
		const page = this.getPage();

		if (!page.url().includes('/settings/library')) {
			await page.goto('/settings/library');
			await page.waitForLoadState('networkidle');
		}

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).toBeVisible();

		const paths = pathsString.split(',').map((p) => resolveServerPath(p.trim()));
		const pathElements = libraryItem.locator('[data-testid^="library-path-value-"]');

		// allTextContents() is a one-shot snapshot; poll so a save that is
		// still round-tripping doesn't flake the assertion.
		for (const expectedPath of paths) {
			await expect
				.poll(() => pathElements.allTextContents(), { timeout: 10_000 })
				.toContain(expectedPath);
		}
	}
);

Then(
	'User cannot see library {string}',
	async function (this: AudiodWorld, libraryName: string) {
		const page = this.getPage();

		await page.goto('/settings/library');
		await page.waitForLoadState('networkidle');

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).not.toBeVisible();
	}
);

Then(
	'Library {string} shows track count > {int}',
	{ timeout: 90000 },
	async function (this: AudiodWorld, libraryName: string, minCount: number) {
		const page = this.getPage();

		if (!page.url().includes('/settings/library')) {
			await page.goto('/settings/library');
			await page.waitForLoadState('networkidle');
		}

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).toBeVisible();

		const scanButton = libraryItem.locator('[data-testid^="scan-library-button-"]');
		await scanButton.click();

		const scanProgress = libraryItem.locator('[data-testid^="scan-progress-bar-"]');
		const scanComplete = page.locator('[data-testid="library-scan-complete"]');

		await expect(async () => {
			const progressVisible = await scanProgress.isVisible();
			const completeVisible = await scanComplete.isVisible();
			expect(progressVisible || completeVisible).toBe(true);
		}).toPass({ timeout: 30000 });

		if (await scanProgress.isVisible()) {
			await expect(scanProgress).not.toBeVisible({ timeout: 60000 });
		}

		let finalTrackCount = 0;
		await expect(async () => {
			const trackCountEl = libraryItem.locator('[data-testid^="library-track-count-"]');
			const text = await trackCountEl.textContent();
			const match = text?.match(/(\d+)/);
			finalTrackCount = match ? parseInt(match[1], 10) : 0;
			expect(finalTrackCount).toBeGreaterThan(minCount);
		}).toPass({ timeout: 10000 });

		this.initialTrackCount = finalTrackCount;
	}
);

Then(
	'Library {string} track count has increased',
	{ timeout: 60000 },
	async function (this: AudiodWorld, libraryName: string) {
		const page = this.getPage();

		if (this.initialTrackCount === undefined) {
			throw new Error('No initial track count stored. Use "Library shows track count > X" step first.');
		}

		if (!page.url().includes('/settings/library')) {
			await page.goto('/settings/library');
			await page.waitForLoadState('networkidle');
		}

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).toBeVisible();

		const previousCount = this.initialTrackCount;

		await expect(async () => {
			const trackCountEl = libraryItem.locator('[data-testid^="library-track-count-"]');
			const text = await trackCountEl.textContent();
			const match = text?.match(/(\d+)/);
			const trackCount = match ? parseInt(match[1], 10) : 0;
			expect(trackCount).toBeGreaterThan(previousCount);
		}).toPass({ timeout: 45000 });
	}
);

Then('User can see library {string}', async function (this: AudiodWorld, libraryName: string) {
	const page = this.getPage();

	await page.goto('/settings/library');
	await page.waitForLoadState('networkidle');

	const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
	await expect(libraryItem).toBeVisible();
});

Then('User cannot delete library {string}', async function (this: AudiodWorld, libraryName: string) {
	const page = this.getPage();

	if (!page.url().includes('/settings/library')) {
		await page.goto('/settings/library');
		await page.waitForLoadState('networkidle');
	}

	const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
	await expect(libraryItem).toBeVisible();

	const deleteButton = libraryItem.locator('[data-testid^="delete-library-button-"]');
	await expect(deleteButton).not.toBeVisible();
});

Then('User cannot edit library {string}', async function (this: AudiodWorld, libraryName: string) {
	const page = this.getPage();

	if (!page.url().includes('/settings/library')) {
		await page.goto('/settings/library');
		await page.waitForLoadState('networkidle');
	}

	const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
	await expect(libraryItem).toBeVisible();

	const editButton = libraryItem.locator('[data-testid^="edit-library-button-"]');
	await expect(editButton).not.toBeVisible();
});

Then('User sees albums in the library', async function (this: AudiodWorld) {
	const page = this.getPage();

	const albumGrid = page.locator('[data-testid="album-grid"]');
	await expect(albumGrid).toBeVisible({ timeout: 10000 });

	const albumCards = page.locator('[data-testid^="album-card-"]');
	const count = await albumCards.count();
	expect(count).toBeGreaterThan(0);
});

Then('User sees albums with cover images', async function (this: AudiodWorld) {
	const page = this.getPage();

	const albumGrid = page.locator('[data-testid="album-grid"]');
	await expect(albumGrid).toBeVisible({ timeout: 10000 });

	const albumCards = page.locator('[data-testid^="album-card-"]');
	const count = await albumCards.count();
	expect(count).toBeGreaterThan(0);

	const coverContainers = page.locator('[data-testid^="album-card-"] .aspect-square');
	const containerCount = await coverContainers.count();
	expect(containerCount).toBeGreaterThan(0);
});

Then('User sees album titles', async function (this: AudiodWorld) {
	const page = this.getPage();

	await expect(page.locator('[data-testid="album-grid"]')).toBeVisible({ timeout: 10000 });

	const albumTitles = page.locator('[data-testid^="album-card-"] h3');
	const count = await albumTitles.count();
	expect(count).toBeGreaterThan(0);

	const firstTitle = await albumTitles.first().textContent();
	expect(firstTitle?.trim().length).toBeGreaterThan(0);
});

Then('User sees album artists', async function (this: AudiodWorld) {
	const page = this.getPage();

	await expect(page.locator('[data-testid="album-grid"]')).toBeVisible({ timeout: 10000 });

	const albumArtists = page.locator('[data-testid^="album-card-"] p');
	const count = await albumArtists.count();
	expect(count).toBeGreaterThan(0);

	const firstArtist = await albumArtists.first().textContent();
	expect(firstArtist?.trim().length).toBeGreaterThan(0);
});

Then('User sees album details page', async function (this: AudiodWorld) {
	const page = this.getPage();

	await page.waitForURL('**/album/**');

	const albumDetails = page.locator('[data-testid="album-details"]');
	await expect(albumDetails).toBeVisible({ timeout: 10000 });
});

Then('User sees track list for the album', { timeout: 15000 }, async function (this: AudiodWorld) {
	const page = this.getPage();

	const trackList = page.locator('[data-testid="track-list"]');
	await expect(trackList).toBeVisible({ timeout: 10000 });

	const tracks = page.locator('[data-testid^="track-row-"]');
	const count = await tracks.count();
	expect(count).toBeGreaterThan(0);
});

Then('Only hi-res albums are visible', async function (this: AudiodWorld) {
	const page = this.getPage();

	await expect
		.poll(
			async () => {
				return await page
					.locator('[data-testid^="album-card-"][data-hires="false"]')
					.count();
			},
			{ timeout: 5000 }
		)
		.toBe(0);

	const hiResCount = await page
		.locator('[data-testid^="album-card-"][data-hires="true"]')
		.count();
	expect(hiResCount).toBeGreaterThan(0);
});

Then('All albums are visible', async function (this: AudiodWorld) {
	const page = this.getPage();

	if (this.initialAlbumCount === undefined) {
		throw new Error(
			'No initial album count stored. Use "Library shows both hi-res and standard albums" first.'
		);
	}

	await expect
		.poll(
			async () => {
				return await page.locator('[data-testid^="album-card-"]').count();
			},
			{ timeout: 5000 }
		)
		.toBe(this.initialAlbumCount);
});

Then('Hi-res filter is enabled', async function (this: AudiodWorld) {
	const page = this.getPage();
	const toggle = page.locator('[data-testid="hi-res-filter-toggle"]');
	await expect(toggle).toHaveAttribute('data-active', 'true');
});

Then('Album grid shows a matching album', async function (this: AudiodWorld) {
	const page = this.getPage();
	const cards = page.locator('[data-testid="album-grid"] [data-testid^="album-card-"]');
	await expect(cards.first()).toBeVisible({ timeout: 5000 });
	expect(await cards.count()).toBeGreaterThan(0);
});

Then('Search results are empty', async function (this: AudiodWorld) {
	const page = this.getPage();
	const empty = page.locator('[data-testid="search-empty"]');
	await expect(empty).toBeVisible({ timeout: 5000 });
});

Then('Search scope tabs are visible', async function (this: AudiodWorld) {
	const page = this.getPage();
	await expect(page.locator('[data-testid="search-scope-tabs"]')).toBeVisible({ timeout: 5000 });
});

Then('Search scope tabs are hidden', async function (this: AudiodWorld) {
	const page = this.getPage();
	await expect(page.locator('[data-testid="search-scope-tabs"]')).toHaveCount(0);
});

Then('Track search results include at least one track', async function (this: AudiodWorld) {
	const page = this.getPage();
	const results = page.locator('[data-testid^="track-search-result-"]');
	await expect(results.first()).toBeVisible({ timeout: 5000 });
	expect(await results.count()).toBeGreaterThan(0);
});

Then('User is redirected to the library browse page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await expect(page).toHaveURL(/\/(\?.*)?$/, { timeout: 5000 });
});

Then('Search input is focused', async function (this: AudiodWorld) {
	const page = this.getPage();
	const focused = await page.evaluate(
		() => document.activeElement?.getAttribute('data-testid') === 'search-input'
	);
	expect(focused).toBe(true);
});

Then(
	'Sort primary is {string} {word}',
	async function (this: AudiodWorld, field: string, dir: string) {
		const page = this.getPage();
		await expect(page.locator('[data-testid="sort-primary-field"]')).toHaveValue(field);
		const expectedDir = dir.startsWith('asc') ? 'asc' : 'desc';
		await expect(page.locator('[data-testid="sort-primary-dir"]')).toHaveAttribute(
			'data-dir',
			expectedDir
		);
	}
);

Then(
	'Sort secondary is {string} {word}',
	async function (this: AudiodWorld, field: string, dir: string) {
		const page = this.getPage();
		await expect(page.locator('[data-testid="sort-secondary-field"]')).toHaveValue(field);
		const expectedDir = dir.startsWith('asc') ? 'asc' : 'desc';
		await expect(page.locator('[data-testid="sort-secondary-dir"]')).toHaveAttribute(
			'data-dir',
			expectedDir
		);
	}
);

Then('URL contains sort {string}', async function (this: AudiodWorld, sortStr: string) {
	const page = this.getPage();
	await expect
		.poll(async () => new URL(page.url()).searchParams.get('sort'), { timeout: 3000 })
		.toBe(sortStr);
});

Then('Album grid is scrolled to the previous position', async function (this: AudiodWorld) {
	const page = this.getPage();

	if (this.savedScrollTop === undefined) {
		throw new Error('No saved scroll position. Use "User has scrolled down in the album grid" first.');
	}

	const expected = this.savedScrollTop;

	await expect
		.poll(
			async () => {
				return await page.evaluate(() => {
					const main = document.querySelector('main');
					return main?.scrollTop ?? 0;
				});
			},
			{ timeout: 5000 }
		)
		.toBeGreaterThan(expected - 5);

	const actual = await page.evaluate(() => document.querySelector('main')?.scrollTop ?? 0);
	expect(Math.abs(actual - expected)).toBeLessThan(5);
});
