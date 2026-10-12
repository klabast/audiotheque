import { expect, type Page } from '@playwright/test';

interface CatalogTrack {
	id: number;
	title: string;
	albumId: number;
	albumTitle: string;
}

/** Every track the logged-in user can see, with its album. */
async function catalog(page: Page): Promise<CatalogTrack[]> {
	const libraries = await (await page.request.get('/api/libraries')).json();
	const tracks: CatalogTrack[] = [];
	for (const library of libraries) {
		const albums = await (await page.request.get(`/api/libraries/${library.id}/albums`)).json();
		for (const album of albums) {
			const albumTracks = await (await page.request.get(`/api/albums/${album.id}/tracks`)).json();
			for (const t of albumTracks) {
				tracks.push({ id: t.id, title: t.title, albumId: album.id, albumTitle: album.title });
			}
		}
	}
	return tracks;
}

export async function findTrack(page: Page, title: string): Promise<CatalogTrack> {
	const track = (await catalog(page)).find((t) => t.title === title);
	if (!track) throw new Error(`Track "${title}" is not in the test library`);
	return track;
}

export async function findAlbumId(page: Page, albumTitle: string): Promise<number> {
	const track = (await catalog(page)).find((t) => t.albumTitle === albumTitle);
	if (!track) throw new Error(`Album "${albumTitle}" is not in the test library`);
	return track.albumId;
}

export async function getSession(page: Page) {
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	return await response.json();
}

export async function trackTitles(page: Page, ids: number[]): Promise<string[]> {
	if (ids.length === 0) return [];
	const response = await page.request.post('/api/tracks/lookup', { data: { ids } });
	expect(response.ok()).toBe(true);
	const tracks: { id: number; title: string }[] = await response.json();
	return ids.map((id) => tracks.find((t) => t.id === id)?.title ?? `#${id}`);
}

/** Replaces the queue with the given tracks, through the API. */
export async function setQueue(page: Page, titles: string[]): Promise<void> {
	expect((await page.request.delete('/api/playback/queue')).ok()).toBe(true);
	const trackIds: number[] = [];
	for (const title of titles) trackIds.push((await findTrack(page, title)).id);
	const response = await page.request.post('/api/playback/queue', {
		data: { trackIds, position: 'end' }
	});
	expect(response.ok()).toBe(true);
}

export function waitForQueueResponse(page: Page) {
	return page.waitForResponse(
		(r) => r.url().includes('/api/playback/queue') && r.status() === 200
	);
}

export async function openQueueSidebar(page: Page): Promise<void> {
	const panel = page.locator('[data-testid="queue-panel"]');
	if ((await panel.count()) > 0 && (await panel.isVisible())) return;
	await page.locator('[data-testid="toggle-queue-button"]').click();
	await panel.waitFor({ state: 'visible' });
}

export function queueEntries(page: Page) {
	return page.locator('[data-testid="queue-explicit"] li[data-testid^="queue-item-"]');
}

export function queueEntry(page: Page, title: string) {
	return queueEntries(page).filter({
		has: page.locator('[data-testid="queue-item-title"]', { hasText: new RegExp(`^${title}$`) })
	});
}
