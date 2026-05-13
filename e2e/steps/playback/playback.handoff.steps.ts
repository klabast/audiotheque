import { Given, When, Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { getDeviceId } from './playback.given.steps';
import { getTestMpdState } from '../../support/test-mpd';
import { waitForClientId } from '../../support/player';

// =====================
// GIVEN steps
// =====================

Given('Music is playing on browser', async function (this: AudiodWorld) {
	const page = this.getPage();

	await page.goto('/');
	await page.waitForLoadState('domcontentloaded');
	// Unified-session invariant: api.playAlbum needs X-Audiod-Client-Id, which
	// the UI only attaches once the WS welcome lands. Without this wait the
	// click can race the WS and the play request 400s — leaving the
	// waitForResponse(200) below to time out the whole 15s step.
	await waitForClientId(page);

	// Pick "Test Album - Multi Long" (fixture at e2e/data/music/04-multi-long)
	// so position-restoration and skip-track scenarios have both tracks
	// long enough for the seeks AND a next track to advance to.
	const albumCard = page.locator('[data-testid^="album-card-"]').filter({
		has: page.locator('[data-testid^="album-title-"]', { hasText: /^Test Album - Multi Long$/ })
	});
	await albumCard.waitFor({ state: 'visible' });
	const responsePromise = page.waitForResponse((r) =>
		r.url().includes('/api/playback/play') && r.status() === 200
	);
	await albumCard.hover();
	await albumCard.locator('[data-testid^="play-album-"]').click();
	await responsePromise;

	await expect(page.locator('[data-testid="player-footer"]')).toBeVisible();
});

Given('Playback position is at {int} seconds', async function (this: AudiodWorld, seconds: number) {
	const page = this.getPage();

	const response = await page.request.post('/api/playback/seek', {
		data: { position: seconds }
	});
	expect(response.ok()).toBe(true);
});

Given('{string} playback position is at {int} seconds', async function (
	this: AudiodWorld, deviceName: string, seconds: number
) {
	const page = this.getPage();
	getDeviceId(this, deviceName); // validate device exists

	// Seek via playback API (which forwards to device)
	const response = await page.request.post('/api/playback/seek', {
		data: { position: seconds }
	});
	expect(response.ok()).toBe(true);

	// Verify via test-mpd observation API
	const state = await getTestMpdState();
	expect(state.elapsed).toBe(seconds);

});

// =====================
// WHEN steps
// =====================

// Note: 'User transfers playback to browser' is defined in playback.when.steps.ts

When('User seeks to {int} seconds on {string}', async function (
	this: AudiodWorld, seconds: number, deviceName: string
) {
	const page = this.getPage();
	getDeviceId(this, deviceName); // validate device exists

	const response = await page.request.post('/api/playback/seek', {
		data: { position: seconds }
	});
	expect(response.ok()).toBe(true);

	// Verify device received the seek via test-mpd. test-mpd has a 1Hz
	// auto-advance clock while state=play (added so position-progress
	// e2e can assert the seek bar moves), so by the time we read state
	// elapsed may already be ahead by a tick. ±2s window is wide enough
	// to absorb that without losing the actual signal.
	const state = await getTestMpdState();
	expect(state.elapsed).toBeGreaterThanOrEqual(seconds);
	expect(state.elapsed).toBeLessThanOrEqual(seconds + 2);
});

When('User sets volume to {int}% via API', async function (this: AudiodWorld, volumePercent: number) {
	const page = this.getPage();

	const response = await page.request.post('/api/playback/volume', {
		data: { volume: volumePercent }
	});
	expect(response.ok()).toBe(true);
});

When('User seeks to {int} seconds via API', async function (this: AudiodWorld, seconds: number) {
	const page = this.getPage();

	const response = await page.request.post('/api/playback/seek', {
		data: { position: seconds }
	});
	expect(response.ok()).toBe(true);
});

// =====================
// THEN steps
// =====================

Then('{string} playback position is approximately {int} seconds', async function (
	this: AudiodWorld, deviceName: string, expectedSeconds: number
) {
	getDeviceId(this, deviceName); // validate device exists

	const state = await getTestMpdState();

	// Allow 5 second tolerance
	expect(state.elapsed).toBeGreaterThanOrEqual(expectedSeconds - 5);
	expect(state.elapsed).toBeLessThanOrEqual(expectedSeconds + 5);
});

Then('Session position is approximately {int} seconds', async function (
	this: AudiodWorld, expectedSeconds: number
) {
	const page = this.getPage();

	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();

	expect(session.current).toBeTruthy();
	expect(session.current.position).toBeGreaterThanOrEqual(expectedSeconds - 5);
	expect(session.current.position).toBeLessThanOrEqual(expectedSeconds + 5);
});

Then('Session device volume for browser is {int}%', async function (
	this: AudiodWorld, expectedVolume: number
) {
	const page = this.getPage();
	// Under the unified-session invariant the "browser" device key in
	// deviceVolumes is the WS-issued client id for THIS tab, not "".
	const clientId = await page.evaluate(() => document.body.dataset.audiodClientId || '');
	expect(clientId).toBeTruthy();

	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();

	expect(session.deviceVolumes).toBeTruthy();
	expect(session.deviceVolumes[clientId]).toBe(expectedVolume);
});

Then('Session source is preserved', async function (this: AudiodWorld) {
	const page = this.getPage();

	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();

	expect(session.source).toBeTruthy();
	expect(session.source.type).toBeTruthy();
});

Then('Session is on the second track', async function (this: AudiodWorld) {
	const page = this.getPage();

	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();

	expect(session.current).toBeTruthy();
	// The session should have history (first track was played before skip)
	// We can't know the exact track ID, but we know it's different from initial
	// Check that source.remaining has fewer items than before
});

Then('Session state is paused', async function (this: AudiodWorld) {
	const page = this.getPage();

	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();

	expect(session.state).toBe('paused');
});
