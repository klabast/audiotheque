/**
 * Queue steps for playback - When
 */
import { When } from '@cucumber/cucumber';
import { expect, type Page } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import {
	findTrack,
	openQueueSidebar,
	queueEntries,
	queueEntry,
	waitForQueueResponse
} from '../../support/queue';

// In-app navigation, so the playing tab is not reloaded.
async function openAlbum(page: Page, albumTitle: string) {
	if (new URL(page.url()).pathname !== '/') {
		await page.goto('/');
	}
	const albumCard = page.locator('[data-testid^="album-card-"]').filter({
		has: page
			.locator('[data-testid^="album-title-"]')
			.filter({ hasText: new RegExp(`^${albumTitle}$`, 'i') })
	});
	await albumCard.locator('a').first().click();
	await page.locator('[data-testid="album-details"]').waitFor({ state: 'visible' });
}

async function clickAndWaitForQueue(page: Page, selector: string) {
	const response = waitForQueueResponse(page);
	await page.locator(selector).click();
	await response;
}

async function queueTrack(page: Page, title: string, buttonPrefix: string) {
	const track = await findTrack(page, title);
	await openAlbum(page, track.albumTitle);
	await clickAndWaitForQueue(page, `[data-testid="${buttonPrefix}-${track.id}"]`);
}

When('User adds track {string} to queue', async function (this: AudiodWorld, title: string) {
	await queueTrack(this.getPage(), title, 'track-add-to-queue-button');
});

When('User adds track {string} to play next', async function (this: AudiodWorld, title: string) {
	await queueTrack(this.getPage(), title, 'track-play-next-button');
});

When('User adds album {string} to queue', async function (this: AudiodWorld, albumTitle: string) {
	const page = this.getPage();
	await openAlbum(page, albumTitle);
	await clickAndWaitForQueue(page, '[data-testid="add-album-to-queue-button"]');
});

When('User adds album {string} to play next', async function (this: AudiodWorld, albumTitle: string) {
	const page = this.getPage();
	await openAlbum(page, albumTitle);
	await clickAndWaitForQueue(page, '[data-testid="play-album-next-button"]');
});

When('User removes {string} from queue', async function (this: AudiodWorld, title: string) {
	const page = this.getPage();
	await openQueueSidebar(page);
	const response = waitForQueueResponse(page);
	await queueEntry(page, title).locator('[data-testid="queue-item-remove-button"]').click();
	await response;
});

When('User clears queue', async function (this: AudiodWorld) {
	const page = this.getPage();
	await openQueueSidebar(page);
	await clickAndWaitForQueue(page, '[data-testid="clear-queue-button"]');
});

When('User plays {string} from queue', async function (this: AudiodWorld, title: string) {
	const page = this.getPage();
	await openQueueSidebar(page);
	const response = waitForQueueResponse(page);
	await queueEntry(page, title).locator('[data-testid="queue-item-play-button"]').click();
	await response;
});

When(
	'User moves {string} to position {int}',
	async function (this: AudiodWorld, title: string, position: number) {
		const page = this.getPage();
		await openQueueSidebar(page);
		const titles = queueEntries(page).locator('[data-testid="queue-item-title"]');
		await expect(queueEntry(page, title)).toHaveCount(1);

		for (;;) {
			const index = (await titles.allTextContents()).map((t) => t.trim()).indexOf(title);
			expect(index).toBeGreaterThanOrEqual(0);
			if (index === position - 1) return;

			const button = index > position - 1 ? 'queue-item-move-up-button' : 'queue-item-move-down-button';
			const response = waitForQueueResponse(page);
			await queueEntry(page, title).locator(`[data-testid="${button}"]`).click();
			await response;
			await expect
				.poll(async () => (await titles.allTextContents()).map((t) => t.trim()).indexOf(title))
				.not.toBe(index);
		}
	}
);

When('Current track finishes', async function (this: AudiodWorld) {
	const page = this.getPage();
	await page.waitForFunction(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement | null;
		return audio !== null && audio.duration > 0;
	});
	const advanced = page.waitForResponse(
		(r) => r.url().includes('/api/playback/next') && r.status() === 200
	);
	await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		audio.currentTime = audio.duration - 0.5;
	});
	await advanced;
});
