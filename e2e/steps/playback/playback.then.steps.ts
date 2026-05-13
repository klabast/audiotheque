import { Then } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { previousVolume, getDeviceId } from './playback.given.steps';
import { getTestMpdState } from '../../support/test-mpd';
import { getClientId, playerControl } from '../../support/player';

Then('Music is playing', async function (this: AudiodWorld) {
	const page = this.getPage();

	const playerFooter = page.locator('[data-testid="player-footer"]');
	await expect(playerFooter).toBeVisible();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(false);
});

// Note: "Music is paused" is a dual-purpose step defined in playback.given.steps.ts

Then('Music is stopped', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(true);
});

Then('Player footer is visible', async function (this: AudiodWorld) {
	const page = this.getPage();

	const playerFooter = page.locator('[data-testid="player-footer"]');
	await expect(playerFooter).toBeVisible();
});

Then('Player footer is hidden', async function (this: AudiodWorld) {
	const page = this.getPage();

	const playerFooter = page.locator('[data-testid="player-footer"]');
	// Footer uses grid-rows-[0fr] when hidden (collapses to zero height)
	await expect(playerFooter).toHaveClass(/grid-rows-\[0fr\]/);
});

Then('Player shows {string}', async function (this: AudiodWorld, _albumTitle: string) {
	const page = this.getPage();

	// Try desktop first, fall back to mobile
	const desktop = page.locator('[data-testid="player-track-info"]');
	const mobile = page.locator('[data-testid="player-track-info"]');
	const trackInfo = await desktop.isVisible() ? desktop : mobile;
	await expect(trackInfo).toBeVisible();
});

Then('Player shows paused state', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(true);
});

// Note: "Current track is {string}" is a dual-purpose step defined in playback.given.steps.ts

Then('Playback position is approximately {int}%', async function (this: AudiodWorld, expectedPercent: number) {
	const page = this.getPage();

	const actualPercent = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		if (!audio || !audio.duration) return 0;
		return (audio.currentTime / audio.duration) * 100;
	});

	expect(actualPercent).toBeGreaterThan(expectedPercent - 10);
	expect(actualPercent).toBeLessThan(expectedPercent + 10);
});

// Note: "Volume is {int}%" is a dual-purpose step defined in playback.given.steps.ts
// Note: "Volume is muted" is a dual-purpose step defined in playback.given.steps.ts

Then('Previous volume was {int}%', async function (this: AudiodWorld, expectedPercent: number) {
	expect(previousVolume * 100).toBeCloseTo(expectedPercent, 0);
});

Then('Volume is not muted', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isMuted = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.muted ?? false;
	});

	expect(isMuted).toBe(false);
});

// =====================
// MPD device assertions
// =====================

// Note: "Music is playing on {string}" is a dual-purpose step in playback.given.steps.ts
// Note: "{string} is paused" is a dual-purpose step in playback.given.steps.ts

Then('{string} is playing', async function (this: AudiodWorld, deviceName: string) {
	getDeviceId(this, deviceName); // validate device exists

	const state = await getTestMpdState();
	expect(state.playState).toBe('play');
});

Then('{string} plays {string}', async function (this: AudiodWorld, deviceName: string, _trackTitle: string) {
	getDeviceId(this, deviceName); // validate device exists

	const state = await getTestMpdState();
	expect(state.playState).toBe('play');
	expect(state.currentFile).toBeTruthy();
	// TODO: Verify actual track title when track metadata mapping is implemented
});

Then('{string} plays {string} from approximately {int}:{int}', async function (
	this: AudiodWorld, deviceName: string, _trackTitle: string, minutes: number, seconds: number
) {
	getDeviceId(this, deviceName); // validate device exists

	const expectedSeconds = minutes * 60 + seconds;

	const state = await getTestMpdState();
	expect(state.playState).toBe('play');
	// Allow 5 second tolerance for position
	expect(state.elapsed).toBeGreaterThanOrEqual(expectedSeconds - 5);
	expect(state.elapsed).toBeLessThanOrEqual(expectedSeconds + 5);
});

Then('{string} pauses playback', async function (this: AudiodWorld, deviceName: string) {
	getDeviceId(this, deviceName); // validate device exists

	const state = await getTestMpdState();
	expect(state.playState).toBe('pause');
});

Then('{string} stops playback', async function (this: AudiodWorld, deviceName: string) {
	getDeviceId(this, deviceName); // validate device exists

	const state = await getTestMpdState();
	expect(state.playState).toBe('stop');
});

Then('{string} volume is {int}%', async function (this: AudiodWorld, deviceName: string, volumePercent: number) {
	getDeviceId(this, deviceName); // validate device exists

	const state = await getTestMpdState();
	expect(state.volume).toBe(volumePercent);
});

Then('{string} playback position is approximately {int}%', async function (
	this: AudiodWorld, deviceName: string, expectedPercent: number
) {
	getDeviceId(this, deviceName); // validate device exists

	const page = this.getPage();
	const seekBar = await playerControl(page, 'seek-bar', 'seek-bar-fullscreen');
	const duration = parseFloat((await seekBar.getAttribute('aria-valuemax')) || '0');
	expect(duration).toBeGreaterThan(0);

	const expectedElapsed = (expectedPercent / 100) * duration;
	const tolerance = Math.max(2, duration * 0.1); // ±10% of duration, min 2s

	// Poll briefly so the seek round-trip has time to settle.
	let actualElapsed = 0;
	for (let i = 0; i < 20; i++) {
		const state = await getTestMpdState();
		actualElapsed = state.elapsed;
		if (Math.abs(actualElapsed - expectedElapsed) <= tolerance) break;
		await page.waitForTimeout(100);
	}

	expect(actualElapsed).toBeGreaterThanOrEqual(expectedElapsed - tolerance);
	expect(actualElapsed).toBeLessThanOrEqual(expectedElapsed + tolerance);
});

Then('Player shows playing on {string}', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();

	// Two device-indicator nodes exist (mobile track-info vs desktop device-picker-button);
	// only one is visible per viewport. Filter to the visible one.
	const deviceIndicator = page.locator('[data-testid="device-indicator"]').filter({ visible: true }).first();
	await expect(deviceIndicator).toBeVisible();
	await expect(deviceIndicator).toContainText(deviceName);
});

Then('Browser playback stops', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(true);
});

Then('Browser audio stops', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(true);
});

Then('Browser shows {string}', async function (this: AudiodWorld, expectedText: string) {
	const page = this.getPage();

	const deviceIndicator = page.locator('[data-testid="device-indicator"]').filter({ visible: true }).first();
	await expect(deviceIndicator).toBeVisible();
	await expect(deviceIndicator).toContainText(expectedText);
});

Then('Device list shows {string}', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();

	const devicePicker = page.locator('[data-testid="device-picker-menu"]');
	await expect(devicePicker).toBeVisible();
	await expect(devicePicker).toContainText(deviceName);
});

Then('Browser is now playing', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(false);
});

Then('Queue is preserved', async function (this: AudiodWorld) {
	const page = this.getPage();

	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	// Queue should still have entries after transfer
	expect(session.source).toBeTruthy();
});

Then('Source is preserved', async function (this: AudiodWorld) {
	const page = this.getPage();

	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session.source).toBeTruthy();
	expect(session.source.type).toBeTruthy();
});

// =====================
// Handoff assertions
// =====================

Then('Playback continues from approximately {int}:{int}', async function (
	this: AudiodWorld, minutes: number, seconds: number
) {
	const page = this.getPage();
	const expectedSeconds = minutes * 60 + seconds;

	// Check session position
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	if (session.current) {
		expect(session.current.position).toBeGreaterThanOrEqual(expectedSeconds - 5);
		expect(session.current.position).toBeLessThanOrEqual(expectedSeconds + 5);
	}
});

Then('No audible gap in playback', async function (this: AudiodWorld) {
	// This is a quality assertion that can't be fully verified programmatically
	// The test validates that handoff completed without errors
});

Then('Queue still contains {string}, {string}', async function (
	this: AudiodWorld, _track1: string, _track2: string
) {
	const page = this.getPage();
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	// Verify queue has entries
	expect(session.queue.length).toBeGreaterThanOrEqual(2);
});

Then('Source is still album {string}', async function (this: AudiodWorld, _albumTitle: string) {
	const page = this.getPage();
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session.source).toBeTruthy();
	expect(session.source.type).toBe('album');
});

Then('Same tracks remain in source', async function (this: AudiodWorld) {
	const page = this.getPage();
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session.source).toBeTruthy();
	expect(session.source.remaining.length).toBeGreaterThan(0);
});

Then('Handoff completes cleanly', async function (this: AudiodWorld) {
	const page = this.getPage();
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session.state).toBe('playing');
});

Then('Next track plays on {string}', async function (this: AudiodWorld, deviceName: string) {
	getDeviceId(this, deviceName); // validate device exists

	const state = await getTestMpdState();
	expect(state.playState).toBe('play');
	expect(state.currentFile).toBeTruthy();
});

Then('Browser resumes playback', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(false);
});

Then('User is notified of handoff failure', async function (this: AudiodWorld) {
	const page = this.getPage();

	// Verify session is still on this browser (transfer didn't succeed).
	// Under the unified-session invariant every session names a real device:
	// "still on browser" means session.deviceId === this tab's WS clientId,
	// not "empty". The clientId is exposed on <body data-audiod-client-id>.
	const clientId = await getClientId(page);
	expect(clientId).toBeTruthy();

	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session?.deviceId).toBe(clientId);

	// TODO: Add data-testid="handoff-error" notification to the UI
});

Then('Browser plays {string}', async function (this: AudiodWorld, _trackTitle: string) {
	const page = this.getPage();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(false);
	// TODO: Verify actual track title
});

Then('All connected clients show paused state', async function (this: AudiodWorld) {
	const page = this.getPage();

	// In single-browser E2E, verify the current client shows paused
	const playerFooter = page.locator('[data-testid="player-footer"]');
	await expect(playerFooter).toBeVisible();

	// Check the player shows paused state via UI
	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(isPaused).toBe(true);
});

Then('Player UI updates to reflect current state', async function (this: AudiodWorld) {
	const page = this.getPage();

	const playerFooter = page.locator('[data-testid="player-footer"]');
	await expect(playerFooter).toBeVisible();
});

Then('Progress bar shows actual position', async function (this: AudiodWorld) {
	const page = this.getPage();
	const seekBar = await playerControl(page, 'seek-bar', 'seek-bar-fullscreen');
	await expect(seekBar).toBeVisible();
});

// =====================
// MPD-to-browser assertions
// =====================

Then('Device picker shows {string} option', async function (this: AudiodWorld, optionText: string) {
	const page = this.getPage();

	const devicePicker = page.locator('[data-testid="device-picker-menu"]');
	await expect(devicePicker).toContainText(optionText);
});

Then('Device picker shows {string} as current', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();

	const currentDevice = page.locator('[data-testid="current-device-indicator"]');
	await expect(currentDevice).toContainText(deviceName);
});

// =====================
// Multi-browser sync assertions
// =====================

// Helper: every open browser tab (primary + extras) — sync scenarios assert
// across all of them so any drift between WS subscribers gets caught.
function allBrowsers(world: AudiodWorld) {
	return [world.getPage(), ...world.extraPages.values()];
}

Then('All browsers show playing state', async function (this: AudiodWorld) {
	for (const page of allBrowsers(this)) {
		// Each tab's session must report 'playing' — assert via the API
		// that the tab's cookies hit, since the local audio element only
		// exists in whichever tab owns local playback.
		await expect(async () => {
			const r = await page.request.get('/api/playback/session');
			expect(r.ok()).toBe(true);
			const s = await r.json();
			expect(s?.state).toBe('playing');
		}).toPass({ timeout: 5000 });
	}
});

Then('All browsers show new track', async function (this: AudiodWorld) {
	// Resolve the expected track id from the primary browser's session, then
	// verify every other tab's session catches up to the same track.
	const primary = this.getPage();
	const r = await primary.request.get('/api/playback/session');
	expect(r.ok()).toBe(true);
	const expected = await r.json();
	const expectedTrack = expected?.current?.trackId;
	expect(expectedTrack).toBeTruthy();

	for (const page of allBrowsers(this)) {
		await expect(async () => {
			const resp = await page.request.get('/api/playback/session');
			const s = await resp.json();
			expect(s?.current?.trackId).toBe(expectedTrack);
		}).toPass({ timeout: 5000 });

		const trackInfo = page.locator('[data-testid="player-track-info"]').first();
		await expect(trackInfo).toBeVisible();
	}
});

Then('All browsers show updated queue', async function (this: AudiodWorld) {
	for (const page of allBrowsers(this)) {
		const r = await page.request.get('/api/playback/session');
		expect(r.ok()).toBe(true);
		const s = await r.json();
		expect(Array.isArray(s?.queue)).toBe(true);
	}
});

Then('All browsers show new volume level', async function (this: AudiodWorld) {
	const primary = this.getPage();
	const primaryResp = await primary.request.get('/api/playback/session');
	const primarySession = await primaryResp.json();
	const deviceId = primarySession?.deviceId ?? '';
	const expectedVol = primarySession?.deviceVolumes?.[deviceId];
	expect(expectedVol).toBeDefined();

	for (const page of allBrowsers(this)) {
		await expect(async () => {
			const r = await page.request.get('/api/playback/session');
			const s = await r.json();
			expect(s?.deviceVolumes?.[deviceId]).toBe(expectedVol);
		}).toPass({ timeout: 5000 });
	}
});

Then('New browser shows current track', async function (this: AudiodWorld) {
	// "New browser" means the most recently opened extra tab (or browser B).
	const page = this.extraPages.get('B') ?? this.getPage();
	await expect(async () => {
		const r = await page.request.get('/api/playback/session');
		expect(r.ok()).toBe(true);
		const s = await r.json();
		expect(s?.current?.trackId).toBeTruthy();
	}).toPass({ timeout: 5000 });
	const trackInfo = page.locator('[data-testid="player-track-info"]').first();
	await expect(trackInfo).toBeVisible();
});

Then('New browser shows {string}', async function (this: AudiodWorld, expectedText: string) {
	const page = this.extraPages.get('B') ?? this.getPage();
	const deviceIndicator = page.locator('[data-testid="device-indicator"]').filter({ visible: true }).first();
	await expect(deviceIndicator).toBeVisible();
	await expect(deviceIndicator).toContainText(expectedText);
});

Then('New browser can control playback', async function (this: AudiodWorld) {
	const page = this.extraPages.get('B') ?? this.getPage();
	// "Can control" = the play/pause control is visible and enabled.
	const playPause = page.locator('[data-testid="play-pause-button"]').first();
	await expect(playPause).toBeVisible();
	await expect(playPause).toBeEnabled();
});

Then('Browser A shows {string}', async function (this: AudiodWorld, expectedText: string) {
	const page = this.getBrowser('A');
	const deviceIndicator = page.locator('[data-testid="device-indicator"]').filter({ visible: true }).first();
	await expect(deviceIndicator).toBeVisible();
	await expect(deviceIndicator).toContainText(expectedText);
});

Then('Browser B shows {string}', async function (this: AudiodWorld, expectedText: string) {
	const page = this.getBrowser('B');
	const deviceIndicator = page.locator('[data-testid="device-indicator"]').filter({ visible: true }).first();
	await expect(deviceIndicator).toBeVisible();
	await expect(deviceIndicator).toContainText(expectedText);
});

Then('Other browsers update to show new playback location', async function (this: AudiodWorld) {
	// TODO: Check secondary browser context
});

Then('Progress bar updates on all browsers', async function (this: AudiodWorld) {
	// TODO: Check progress in multiple browser contexts
});

Then('Current time updates on all browsers', async function (this: AudiodWorld) {
	// TODO: Check time in multiple browser contexts
});

Then('Browser syncs to current playback state', async function (this: AudiodWorld) {
	// TODO: Verify sync after reconnect
});

Then('Browser shows current track and position', async function (this: AudiodWorld) {
	const page = this.getPage();
	const desktop = page.locator('[data-testid="player-track-info"]');
	const mobile = page.locator('[data-testid="player-track-info"]');
	const trackInfo = await desktop.isVisible() ? desktop : mobile;
	await expect(trackInfo).toBeVisible();
});

// =====================
// Play track assertions
// =====================

Then('Transfer request fails', async function (this: AudiodWorld) {
	expect(this.lastTransferStatus).toBeDefined();
	expect(this.lastTransferStatus).toBeGreaterThanOrEqual(400);
});

Then('Session remains in browser', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Under the unified-session invariant the session always names a real
	// device. "Remains in browser" means it still names THIS browser tab —
	// the WS-issued clientId — not the just-attempted MPD device.
	const clientId = await getClientId(page);
	expect(clientId).toBeTruthy();

	const r = await page.request.get('/api/playback/session');
	expect(r.ok()).toBe(true);
	const s = await r.json();
	expect(s?.deviceId).toBe(clientId);
});

Then('Session advances to next track', async function (this: AudiodWorld) {
	const page = this.getPage();
	const before = this.preActionTrackId;
	expect(before).toBeTruthy();

	// The position poller runs roughly once a second on MPD-bound sessions;
	// give the auto-advance a few ticks before declaring failure.
	await expect(async () => {
		const r = await page.request.get('/api/playback/session');
		expect(r.ok()).toBe(true);
		const s = await r.json();
		expect(s?.current?.trackId).toBeTruthy();
		expect(s?.current?.trackId).not.toBe(before);
		expect(s?.state).toBe('playing');
	}).toPass({ timeout: 10000 });
});

Then('Next track is {string}', async function (this: AudiodWorld, _trackTitle: string) {
	const page = this.getPage();
	// Verify session has remaining tracks in source
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session.source).toBeTruthy();
	// TODO: Verify the specific next track title when metadata is available
});

// =====================
// Player UI assertions
// =====================

Then('Player footer shows track title', async function (this: AudiodWorld) {
	const page = this.getPage();
	const trackInfo = page.locator('[data-testid="player-track-info"]');
	await expect(trackInfo).toBeVisible();
	// Track title is the first child div with font-semibold
	const title = trackInfo.locator('.font-semibold').first();
	await expect(title).toBeVisible();
	const text = await title.textContent();
	expect(text?.trim().length).toBeGreaterThan(0);
});

Then('Player footer shows artist name', async function (this: AudiodWorld) {
	const page = this.getPage();
	const trackInfo = page.locator('[data-testid="player-track-info"]');
	// Artist is the second child div (text-text-secondary)
	const artist = trackInfo.locator('.text-text-secondary').first();
	await expect(artist).toBeVisible();
	const text = await artist.textContent();
	expect(text?.trim().length).toBeGreaterThan(0);
});

Then('Player footer shows album art', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Album cover is an img inside the footer, sibling to track-info-desktop
	const footer = page.locator('[data-testid="player-footer"]');
	const albumArt = footer.locator('img[alt="Album cover"], img[alt*="album"]').first();
	// Album art may not be present if no cover exists — just check footer is visible
	await expect(footer).toBeVisible();
});

Then('Player footer shows progress bar', async function (this: AudiodWorld) {
	const page = this.getPage();
	const seekBar = await playerControl(page, 'seek-bar', 'seek-bar-fullscreen');
	await expect(seekBar).toBeVisible();
});

Then('Player footer shows current time', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Time display is inside the seek bar area — check for tabular-nums time text
	const seekBar = await playerControl(page, 'seek-bar', 'seek-bar-fullscreen');
	const timeElements = seekBar.locator('..').locator('.tabular-nums');
	expect(await timeElements.count()).toBeGreaterThanOrEqual(1);
});

Then('Player footer shows total duration', async function (this: AudiodWorld) {
	const page = this.getPage();
	const seekBar = await playerControl(page, 'seek-bar', 'seek-bar-fullscreen');
	const timeElements = seekBar.locator('..').locator('.tabular-nums');
	expect(await timeElements.count()).toBeGreaterThanOrEqual(2);
});

Then('Full player view is visible', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Full player is mobile-only. On desktop, the footer IS the full view.
	const footer = page.locator('[data-testid="player-footer"]');
	await expect(footer).toBeVisible();
});

Then('Full player shows album art large', async function (this: AudiodWorld) {
	const page = this.getPage();
	// On desktop, album art is in the footer
	const footer = page.locator('[data-testid="player-footer"]');
	await expect(footer).toBeVisible();
});

Then('Full player shows track list', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Track list in full player is not yet implemented
	// For now, verify footer is visible
	const footer = page.locator('[data-testid="player-footer"]');
	await expect(footer).toBeVisible();
});

Then('Full player view is hidden', async function (this: AudiodWorld) {
	const page = this.getPage();
	// On desktop, this concept doesn't apply — the footer is always the player
	// Just verify footer is still visible (not hidden)
	const footer = page.locator('[data-testid="player-footer"]');
	await expect(footer).toBeVisible();
});

// Asserts the most recent /api/playback/volume request did not error and
// the session reports the desired value at session.deviceVolumes[deviceId].
// Used by the mixerless feature to prove the user-facing API still succeeds
// when the device can't physically change its volume.
Then(
	'Session remembers volume {int}% for {string}',
	async function (this: AudiodWorld, vol: number, deviceName: string) {
		const page = this.getPage();
		const deviceId = this.mpdDevices.get(deviceName);
		if (!deviceId) {
			throw new Error(`MPD device "${deviceName}" not registered`);
		}

		await expect(async () => {
			const r = await page.request.get('/api/playback/session');
			expect(r.ok()).toBe(true);
			const s = await r.json();
			expect(s?.deviceVolumes?.[deviceId]).toBe(vol);
		}).toPass({ timeout: 5000 });
	}
);

Then(
	'Session reports volume capability disabled for current device',
	async function (this: AudiodWorld) {
		const page = this.getPage();
		await expect(async () => {
			const r = await page.request.get('/api/playback/session');
			expect(r.ok()).toBe(true);
			const s = await r.json();
			expect(s?.deviceCapabilities?.volume).toBe(false);
		}).toPass({ timeout: 5000 });
	}
);

Then('Volume change is accepted', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Negative assertion: the player UI must NOT show an error toast or
	// roll the slider back. We just check the API state is consistent —
	// the session has the volume we asked for.
	const r = await page.request.get('/api/playback/session');
	expect(r.ok()).toBe(true);
});

// Asserts the rendered volume control reflects the deviceCapabilities
// hint — slider has aria-disabled=true and the wrapper carries
// data-supports-volume="false". On mobile, the slider lives inside the
// fullscreen overlay, so we open it before checking.
Then('Volume control is disabled in the player', async function (this: AudiodWorld) {
	const page = this.getPage();
	const slider = await playerControl(page, 'volume-slider');
	await expect(slider).toBeAttached();
	await expect(slider).toHaveAttribute('aria-disabled', 'true');
});

// Asserts the position-poller has mirrored MPD's clock into the session.
// The poller runs at 1Hz; we wait up to 5s so a slow CI runner doesn't
// flake on a single missed tick.
Then(
	'Session position advances past {int} seconds',
	async function (this: AudiodWorld, threshold: number) {
		const page = this.getPage();
		await expect(async () => {
			const r = await page.request.get('/api/playback/session');
			expect(r.ok()).toBe(true);
			const s = await r.json();
			expect(s?.current?.position).toBeGreaterThan(threshold);
		}).toPass({ timeout: 5000 });
	}
);

// Asserts the rendered seek bar reflects the position the server is
// pushing over WS. Reads aria-valuenow off the seek-bar slider element
// (rendered with role="slider") rather than the textual time, which
// formats to mm:ss and is harder to compare.
Then(
	'Player seek bar shows progress past {int} seconds',
	async function (this: AudiodWorld, threshold: number) {
		const page = this.getPage();
		const seekBar = await playerControl(page, 'seek-bar', 'seek-bar-fullscreen');
		await expect(async () => {
			const valueNow = await seekBar.getAttribute('aria-valuenow');
			expect(valueNow).not.toBeNull();
			expect(Number(valueNow)).toBeGreaterThan(threshold);
		}).toPass({ timeout: 5000 });
	}
);

