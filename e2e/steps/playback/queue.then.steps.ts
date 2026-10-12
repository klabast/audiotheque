/**
 * Queue steps for playback - Then
 */
import { Then } from '@cucumber/cucumber';
import { expect, type Page } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { getSession, openQueueSidebar, queueEntry, trackTitles } from '../../support/queue';

async function expectQueueOrder(page: Page, titles: string[]) {
	await openQueueSidebar(page);
	await expect(
		page.locator('[data-testid="queue-explicit"] [data-testid="queue-item-title"]')
	).toHaveText(titles);
}

Then('Queue order is {string}', async function (this: AudiodWorld, t1: string) {
	await expectQueueOrder(this.getPage(), [t1]);
});

Then('Queue order is {string}, {string}', async function (this: AudiodWorld, t1: string, t2: string) {
	await expectQueueOrder(this.getPage(), [t1, t2]);
});

Then(
	'Queue order is {string}, {string}, {string}',
	async function (this: AudiodWorld, t1: string, t2: string, t3: string) {
		await expectQueueOrder(this.getPage(), [t1, t2, t3]);
	}
);

Then('Queue is empty', async function (this: AudiodWorld) {
	const page = this.getPage();
	await expect.poll(async () => (await getSession(page)).queue.length).toBe(0);
	await openQueueSidebar(page);
	await expect(page.locator('[data-testid="queue-explicit"]')).toHaveCount(0);
});

Then('Queue shows {string} from album {string}', async function (this: AudiodWorld, title: string, album: string) {
	const page = this.getPage();
	await openQueueSidebar(page);
	await expect(queueEntry(page, title).locator('[data-testid="queue-item-origin"]')).toContainText(album);
});

Then('Source remains album {string}', async function (this: AudiodWorld, albumTitle: string) {
	const page = this.getPage();
	const session = await getSession(page);
	expect(session.source.type).toBe('album');
	const album = await (await page.request.get(`/api/albums/${session.source.id}`)).json();
	expect(album.title).toBe(albumTitle);
});

Then('Now playing is {string}', async function (this: AudiodWorld, title: string) {
	const page = this.getPage();
	await expect
		.poll(async () => {
			const session = await getSession(page);
			return (await trackTitles(page, [session.current?.trackId ?? 0]))[0];
		})
		.toBe(title);
});

Then('Next from source is {string}', async function (this: AudiodWorld, title: string) {
	const page = this.getPage();
	const session = await getSession(page);
	expect(await trackTitles(page, session.source.remaining.slice(0, 1))).toEqual([title]);
});
