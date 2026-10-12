/**
 * Stats steps - Given
 */
import { Given } from '@cucumber/cucumber';
import { expect, type Page } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { findTrack, setQueue } from '../../support/queue';

async function playNext(page: Page) {
	expect((await page.request.post('/api/playback/next')).ok()).toBe(true);
}

/** Starts the track through the queue, so whatever was playing is left behind. */
async function startTrack(page: Page, title: string): Promise<number> {
	await setQueue(page, [title]);
	await playNext(page);
	const { id } = await findTrack(page, title);
	const response = await page.request.post('/api/tracks/lookup', { data: { ids: [id] } });
	expect(response.ok()).toBe(true);
	const [track] = await response.json();
	return track.duration;
}

Given(
	'User has listened to {string} {int} times',
	async function (this: AudiodWorld, title: string, times: number) {
		const page = this.getPage();
		for (let i = 0; i < times; i++) {
			const durationMs = await startTrack(page, title);
			const mostOfIt = Math.ceil((durationMs / 1000) * 0.6);
			expect(
				(await page.request.post('/api/playback/seek', { data: { position: mostOfIt } })).ok()
			).toBe(true);
			await playNext(page);
		}
	}
);

Given('User has skipped {string}', async function (this: AudiodWorld, title: string) {
	const page = this.getPage();
	await startTrack(page, title);
	await playNext(page);
});
