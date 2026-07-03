import { When } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { resolveServerPath } from '../../support/audiod-cli';

When('User navigates to home page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/');
	await page.waitForLoadState('networkidle');
});

When('User clicks library settings link', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.locator('[data-testid="library-settings-link"]').click();
	await page.waitForLoadState('networkidle');
});

When('User navigates to library settings page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/settings/library');
	await page.waitForLoadState('networkidle');
});

When(
	'User creates library named {string} with path(s) {string}',
	{ timeout: 30000 },
	async function (this: AudiodWorld, libraryName: string, pathsString: string) {
		const page = this.getPage();

		await page.goto('/settings/library');
		await page.waitForLoadState('networkidle');

		await page.locator('[data-testid="create-library-button"]').click();

		await page.locator('[data-testid="library-name-input"]').fill(libraryName);

		const paths = pathsString.split(',').map((p) => p.trim());
		for (let i = 0; i < paths.length; i++) {
			if (i > 0) {
				await page.locator('[data-testid="add-path-button"]').click();
			}
			await page.locator(`[data-testid="library-path-input-${i}"]`).fill(resolveServerPath(paths[i]));
		}

		// Wait for the create POST to complete before returning — without this,
		// a follow-up "User can see library X" can race the response and find
		// no row in the list. (Manifested as a flake in larger e2e batches.)
		const createPost = page.waitForResponse(
			(r) => r.url().includes('/api/libraries') && r.request().method() === 'POST',
			{ timeout: 10000 }
		);
		await page.locator('[data-testid="save-library-button"]').click();
		await createPost.catch(() => {
			// Surfaced by the following Then assertion if it never fires.
		});

		this.createdLibraryName = libraryName;
	}
);

When(
	'User deletes library {string}',
	{ timeout: 30000 },
	async function (this: AudiodWorld, libraryName: string) {
		const page = this.getPage();

		if (!page.url().includes('/settings/library')) {
			await page.goto('/settings/library');
			await page.waitForLoadState('networkidle');
		}

		await expect(page.locator('text=Loading libraries...')).not.toBeVisible({ timeout: 10000 });

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).toBeVisible({ timeout: 10000 });

		const deleteButton = libraryItem.locator('[data-testid^="delete-library-button-"]');
		await deleteButton.click();

		await expect(page.locator('[data-testid="delete-library-modal"]')).toBeVisible();

		await page.locator('[data-testid="delete-library-confirmation-input"]').fill(libraryName);

		const confirmButton = page.locator('[data-testid="delete-library-confirm-button"]');
		await expect(confirmButton).toBeEnabled({ timeout: 5000 });

		await confirmButton.click();

		await expect(page.locator('[data-testid="delete-library-modal"]')).not.toBeVisible();
	}
);

When(
	'User renames library {string} to {string}',
	{ timeout: 30000 },
	async function (this: AudiodWorld, oldName: string, newName: string) {
		const page = this.getPage();

		if (!page.url().includes('/settings/library')) {
			await page.goto('/settings/library');
			await page.waitForLoadState('networkidle');
		}

		const libraryItem = page.locator(`[data-testid="library-item-${oldName}"]`);
		await expect(libraryItem).toBeVisible({ timeout: 10000 });

		const editButton = libraryItem.locator('[data-testid^="edit-library-button-"]');
		await editButton.click();

		await expect(page.locator('[data-testid="edit-library-modal"]')).toBeVisible();

		const nameInput = page.locator('[data-testid="edit-library-name-input"]');
		await nameInput.clear();
		if (newName) {
			await nameInput.fill(newName);
		}

		if (!newName) {
			return;
		}

		await page.locator('[data-testid="edit-library-save-button"]').click();
		await expect(page.locator('[data-testid="edit-library-modal"]')).not.toBeVisible();
		await page.locator(`[data-testid="library-item-${newName}"]`).waitFor();
	}
);

When(
	'User edits library {string} adding path {string}',
	{ timeout: 30000 },
	async function (this: AudiodWorld, libraryName: string, newPath: string) {
		const page = this.getPage();

		if (!page.url().includes('/settings/library')) {
			await page.goto('/settings/library');
			await page.waitForLoadState('networkidle');
		}

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).toBeVisible({ timeout: 10000 });

		const editButton = libraryItem.locator('[data-testid^="edit-library-button-"]');
		await editButton.click();

		await expect(page.locator('[data-testid="edit-library-modal"]')).toBeVisible();

		await page.locator('[data-testid="edit-add-path-button"]').click();

		const pathInputs = page.locator('[data-testid^="edit-library-path-input-"]');
		const count = await pathInputs.count();
		await pathInputs.nth(count - 1).fill(resolveServerPath(newPath));

		await page.locator('[data-testid="edit-library-save-button"]').click();
		await expect(page.locator('[data-testid="edit-library-modal"]')).not.toBeVisible();
	}
);

When(
	'User edits library {string} removing path {string}',
	{ timeout: 30000 },
	async function (this: AudiodWorld, libraryName: string, pathToRemove: string) {
		const page = this.getPage();

		if (!page.url().includes('/settings/library')) {
			await page.goto('/settings/library');
			await page.waitForLoadState('networkidle');
		}

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).toBeVisible({ timeout: 10000 });

		const editButton = libraryItem.locator('[data-testid^="edit-library-button-"]');
		await editButton.click();

		await expect(page.locator('[data-testid="edit-library-modal"]')).toBeVisible();

		const absolutePath = resolveServerPath(pathToRemove);
		const pathInputs = page.locator('[data-testid^="edit-library-path-input-"]');
		const count = await pathInputs.count();

		for (let i = 0; i < count; i++) {
			const value = await pathInputs.nth(i).inputValue();
			if (value === absolutePath) {
				await page.locator(`[data-testid="edit-remove-path-button-${i}"]`).click();
				break;
			}
		}

		await page.locator('[data-testid="edit-library-save-button"]').click();
		await expect(page.locator('[data-testid="edit-library-modal"]')).not.toBeVisible();
	}
);

When(
	'User triggers scan for library {string}',
	{ timeout: 30000 },
	async function (this: AudiodWorld, libraryName: string) {
		const page = this.getPage();

		if (!page.url().includes('/settings/library')) {
			await page.goto('/settings/library');
			await page.waitForLoadState('networkidle');
		}

		const libraryItem = page.locator(`[data-testid="library-item-${libraryName}"]`);
		await expect(libraryItem).toBeVisible({ timeout: 10000 });

		const scanButton = libraryItem.locator('[data-testid^="scan-library-button-"]');
		await scanButton.click();
	}
);

When('User navigates to library browse', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.goto('/');
	await page.waitForLoadState('networkidle');
});

When('User clicks on an album', async function (this: AudiodWorld) {
	const page = this.getPage();

	const firstAlbum = page.locator('[data-testid^="album-card-"]').first();
	await firstAlbum.click();

	await page.waitForLoadState('networkidle');
});

When('User opens the last album in the grid', async function (this: AudiodWorld) {
	const page = this.getPage();

	const lastAlbum = page.locator('[data-testid^="album-card-"]').last();
	await lastAlbum.click();

	await page.waitForLoadState('networkidle');
});

When('User enables the hi-res filter', async function (this: AudiodWorld) {
	const page = this.getPage();
	const toggle = page.locator('[data-testid="hi-res-filter-toggle"]');
	await expect(toggle).toBeVisible();
	const active = (await toggle.getAttribute('data-active')) === 'true';
	if (!active) {
		await toggle.click();
	}
	await expect(toggle).toHaveAttribute('data-active', 'true');
});

When('User disables the hi-res filter', async function (this: AudiodWorld) {
	const page = this.getPage();
	const toggle = page.locator('[data-testid="hi-res-filter-toggle"]');
	const active = (await toggle.getAttribute('data-active')) === 'true';
	if (active) {
		await toggle.click();
	}
	await expect(toggle).toHaveAttribute('data-active', 'false');
});

When('User refreshes the page', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.reload();
	await page.waitForLoadState('networkidle');
});

When('User sets primary sort to {string}', async function (this: AudiodWorld, field: string) {
	const page = this.getPage();
	await page.locator('[data-testid="sort-primary-field"]').selectOption(field);
});

When('User sets secondary sort to {string}', async function (this: AudiodWorld, field: string) {
	const page = this.getPage();
	await page.locator('[data-testid="sort-secondary-field"]').selectOption(field);
});

When('User toggles primary sort direction', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.locator('[data-testid="sort-primary-dir"]').click();
});

When('User searches for {string}', async function (this: AudiodWorld, query: string) {
	const page = this.getPage();
	const input = page.locator('[data-testid="search-input"]');
	await input.click();
	await input.fill(query);
});

When('User selects the {string} search scope', async function (this: AudiodWorld, scope: string) {
	const page = this.getPage();
	await page.locator(`[data-testid="search-scope-${scope}"]`).click();
});

When('User clears the search', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.locator('[data-testid="search-clear-button"]').click();
});

When('User presses the search shortcut', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.keyboard.press('Meta+k');
});

When('User presses the slash key', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.locator('body').click({ position: { x: 10, y: 10 } });
	await page.keyboard.press('/');
});

When('User navigates back to library', async function (this: AudiodWorld) {
	const page = this.getPage();

	// Use the back button when it's available (album page) so SvelteKit
	// drives a popstate-style navigation and the home page's snapshot.restore
	// is invoked. `page.goto('/')` is a fresh navigation that bypasses the
	// snapshot mechanism, which silently breaks scroll-restoration tests.
	const backButton = page.locator('[data-testid="back-to-library-button"]');
	try {
		await backButton.waitFor({ state: 'visible', timeout: 5000 });
		await backButton.click();
	} catch {
		await page.goto('/');
	}

	await page.waitForLoadState('networkidle');
});
