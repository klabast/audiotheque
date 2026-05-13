import { After, Given } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import {
	createDevice,
	createDeviceAt,
	disableTestMpdMixer,
	resetTestMpd,
	getTestMpdState
} from '../../support/test-mpd';
import { playerControl, waitForClientId } from '../../support/player';
import { getSharedBrowser, getActiveDeviceConfig, getBaseURL } from '../../support/hooks';

// Store previous volume for mute/unmute tests (shared with then steps via export)
export let previousVolume = 1;
export function setPreviousVolume(v: number) { previousVolume = v; }

// Helper: get device ID from world, throw if not found
function getDeviceId(world: AudiodWorld, deviceName: string): string {
	const deviceId = world.mpdDevices.get(deviceName);
	if (!deviceId) {
		throw new Error(`MPD device "${deviceName}" not set up. Use 'Given MPD device "${deviceName}" is available' first.`);
	}
	return deviceId;
}

// Export for use in then/when steps
export { getDeviceId };

Given('User is playing album {string}', async function (this: AudiodWorld, albumTitle: string) {
	const page = this.getPage();

	await page.goto('/');
	await page.waitForLoadState('domcontentloaded');
	// Unified-session invariant: api.playAlbum needs X-Audiod-Client-Id, which
	// the UI only attaches once the WS welcome lands. Without this wait the
	// click can race the WS and the play request 400s.
	await waitForClientId(page);

	const albumCard = page.locator('[data-testid^="album-card-"]').filter({
		has: page.locator('[data-testid^="album-title-"]').filter({ hasText: new RegExp(`^${albumTitle}$`, 'i')})
	});

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/play') && response.status() === 200
	);

	await albumCard.hover();
	const playButton = albumCard.locator('[data-testid^="play-album-"]');
	await playButton.click();

	await responsePromise;

	await expect(page.locator('[data-testid="player-footer"]')).toBeVisible();
});

// Dual-purpose step: ensures state for Given, asserts for Then
Given('Music is paused', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isPaused = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});

	if (!isPaused) {
		// Use desktop play-pause button (mobile one is hidden on desktop viewport)
		const btn = page.locator('[data-testid="play-pause-button"]');
		const responsePromise = page.waitForResponse((response) =>
			response.url().includes('/api/playback/pause') && response.status() === 200
		);
		await btn.click();
		await responsePromise;
	}

	const finalState = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.paused ?? true;
	});
	expect(finalState).toBe(true);
});

Given('Current track is {string}', async function (this: AudiodWorld, _trackTitle: string) {
	const page = this.getPage();
	await expect(page.locator('[data-testid="player-footer"]')).toBeVisible();
});

Given('Previous track was {string}', async function (this: AudiodWorld, _trackTitle: string) {
	// Informational - sets expectation for what previous() should return to
});

// Dual-purpose step: ensures state for Given, asserts for Then
Given('Volume is {int}%', async function (this: AudiodWorld, volumePercent: number) {
	const page = this.getPage();

	// Use keyboard on volume slider (each ArrowRight/Left = 10%)
	const volumeSlider = await playerControl(page, 'volume-slider');
	await volumeSlider.focus();

	const currentValue = await volumeSlider.getAttribute('aria-valuenow');
	const currentPercent = Math.round(parseFloat(currentValue || '100'));
	const diff = volumePercent - currentPercent;
	const steps = Math.abs(Math.round(diff / 10));
	const key = diff > 0 ? 'ArrowRight' : 'ArrowLeft';

	for (let i = 0; i < steps; i++) {
		await volumeSlider.press(key);
		await page.waitForTimeout(50);
	}

	previousVolume = volumePercent / 100;

	// Verify via aria-valuenow (works for both browser and remote)
	const newValue = await volumeSlider.getAttribute('aria-valuenow');
	expect(Math.round(parseFloat(newValue || '0'))).toBeCloseTo(volumePercent, 0);
});

// Dual-purpose step: ensures state for Given, asserts for Then
Given('Volume is muted', async function (this: AudiodWorld) {
	const page = this.getPage();

	const isMuted = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.muted ?? false;
	});

	if (!isMuted) {
		const muteButton = await playerControl(page, 'mute-button');
		await muteButton.click();
	}

	const finalState = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.muted ?? false;
	});
	expect(finalState).toBe(true);
});

// =====================
// MPD device steps
// =====================

Given('An unreachable MPD device {string} is registered', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();
	// 127.0.0.1:1 is reliably refused — TCP connect fails fast, which is what
	// "MPD is unreachable" looks like to the device resolver.
	const deviceId = await createDeviceAt(page, deviceName, '127.0.0.1:1');
	this.mpdDevices.set(deviceName, deviceId);
});

Given('MPD device {string} is available', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();

	// Reset test-mpd state for a clean scenario
	await resetTestMpd();

	// Register device via real settings API (authenticated page context)
	const deviceId = await createDevice(page, deviceName);

	this.mpdDevices.set(deviceName, deviceId);
});

// Toggle the test-mpd mixer off so subsequent setvol commands return the
// "No mixer" ACK that real MPD returns when configured with
// `mixer_type "none"` (HiFiBerry's default).
Given('{string} has no mixer', async function (this: AudiodWorld, deviceName: string) {
	// Resolves the device first so the step fails clearly when called for an
	// unregistered device — and to keep the deviceName parameter meaningful
	// even though the test-mpd flag is global.
	getDeviceId(this, deviceName);
	await disableTestMpdMixer();
});

// Dual-purpose: ensures music is playing on a specific MPD device.
// We deliberately pick "Test Album - Multi Long" (2 tracks × 90s, fixture
// at e2e/data/music/04-multi-long) so handoff scenarios both:
//   - seek to 30/45/60/75/80 seconds without overrunning the track and
//     triggering an `ended` event in the browser audio element (which
//     would fan out to /api/playback/next and clear session.Current)
//   - have a "next track" available so skip-then-transfer scenarios land
//     on the second track instead of an empty session.
Given('Music is playing on {string}', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();
	const deviceId = getDeviceId(this, deviceName);

	await page.goto('/');
	await page.waitForLoadState('domcontentloaded');
	await waitForClientId(page);

	const albumCard = page.locator('[data-testid^="album-card-"]').filter({
		has: page.locator('[data-testid^="album-title-"]', { hasText: /^Test Album - Multi Long$/ })
	});
	await albumCard.waitFor({ state: 'visible', timeout: 10000 });
	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/play') && response.status() === 200
	);
	await albumCard.hover();
	const playButton = albumCard.locator('[data-testid^="play-album-"]');
	await playButton.click();
	await responsePromise;

	// Given steps may use API per feedback_e2e_rules.md (CLI/API allowed in
	// setup; only When steps must drive the UI). Direct API is dramatically
	// faster than opening the picker — important here because the 15s
	// per-step budget barely fits the whole Given on slow CI runners.
	const transferResponse = await page.request.post('/api/playback/transfer', {
		data: { deviceId: deviceId }
	});
	expect(transferResponse.ok()).toBe(true);

	// Verify device is playing via test-mpd observation API
	const state = await getTestMpdState();
	expect(state.playState).toBe('play');

	// The position-poller runs at 1 Hz on the server. It must observe
	// state=play at least once before a subsequent state=stop counts as
	// a real track-end (see PollMPDPositions state machine). Without
	// this wait, /track-ended in the next step can race in before
	// observedPlay is latched, and the auto-advance scenario fails.
	await page.waitForTimeout(1500);

	await expect(page.locator('[data-testid="player-footer"]')).toBeVisible();
});

Given('User is playing album {string} in browser', async function (this: AudiodWorld, albumTitle: string) {
	const page = this.getPage();

	await page.goto('/');
	await page.waitForLoadState('domcontentloaded');
	await waitForClientId(page);

	const albumCard = page.locator('[data-testid^="album-card-"]').filter({
		has: page.locator('[data-testid^="album-title-"]').filter({ hasText: new RegExp(`^${albumTitle}$`, 'i')})
	});

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/play') && response.status() === 200
	);
	await albumCard.hover();
	const playButton = albumCard.locator('[data-testid^="play-album-"]');
	await playButton.click();
	await responsePromise;

	await expect(page.locator('[data-testid="player-footer"]')).toBeVisible();
});

Given('Current track is {string} at {int}:{int}', async function (
	this: AudiodWorld, _trackTitle: string, minutes: number, seconds: number
) {
	const page = this.getPage();
	const position = minutes * 60 + seconds;

	// Seek to the specified position via API
	const response = await page.request.post('/api/playback/seek', {
		data: { position }
	});
	expect(response.ok()).toBe(true);

	await expect(page.locator('[data-testid="player-footer"]')).toBeVisible();
});

// Dual-purpose: ensures/asserts MPD device is paused
Given('{string} is paused', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();
	getDeviceId(this, deviceName); // validate device exists

	const state = await getTestMpdState();

	if (state.playState !== 'pause') {
		const btn = page.locator('[data-testid="play-pause-button"]');
		const responsePromise = page.waitForResponse((response) =>
			response.url().includes('/api/playback/pause') && response.status() === 200
		);
		await btn.click();
		await responsePromise;
	}

	// Verify paused
	const verifyState = await getTestMpdState();
	expect(verifyState.playState).toBe('pause');
});

Given('Current track is about to end', async function (this: AudiodWorld) {
	const page = this.getPage();

	// Seek near the end of the current track
	await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		if (audio && audio.duration > 0) {
			audio.currentTime = audio.duration - 2; // 2 seconds before end
		}
	});
});

Given('Queue contains {string}, {string}', async function (
	this: AudiodWorld, _track1: string, _track2: string
) {
	// TODO: Add tracks to queue via API when queue management is implemented
	return 'pending';
});

Given('Queue contains tracks', async function (this: AudiodWorld) {
	// Verify session has a source with remaining tracks
	const page = this.getPage();
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session.source).toBeTruthy();
});

Given('Source is album {string} with tracks remaining', async function (
	this: AudiodWorld, _albumTitle: string
) {
	const page = this.getPage();
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session.source).toBeTruthy();
	expect(session.source.type).toBe('album');
	expect(session.source.remaining.length).toBeGreaterThan(0);
});

Given('Source is album {string}', async function (this: AudiodWorld, _albumTitle: string) {
	const page = this.getPage();
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session.source).toBeTruthy();
	expect(session.source.type).toBe('album');
});

Given('{string} becomes unavailable during handoff', async function (
	this: AudiodWorld, deviceName: string
) {
	getDeviceId(this, deviceName); // validate device exists

	// Stop the test-mpd to simulate device becoming unavailable
	// In docker-compose, we'd stop the container. For now, we can't easily
	// simulate this without container control. The transfer will fail because
	// the device resolver will get a connection error.
	// TODO: Add a /shutdown endpoint to test-mpd or use docker stop
});

Given('Another client changes {string} state', async function (
	this: AudiodWorld, deviceName: string
) {
	getDeviceId(this, deviceName); // validate device exists

	// Verify the device is reachable via test-mpd
	const state = await getTestMpdState();
	expect(state.playState).toBeTruthy();
});

// =====================
// Player UI steps
// =====================

Given('No music is playing', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Verify no active session
	const response = await page.request.get('/api/playback/session');
	if (response.ok()) {
		try {
			const session = await response.json();
			expect(session?.current).toBeFalsy();
		} catch {
			// Empty response = no session, which is what we want
		}
	}
});

// =====================
// Album details / play track steps
// =====================

Given('User is on album details page for {string}', async function (this: AudiodWorld, albumTitle: string) {
	const page = this.getPage();

	// Navigate to library and wait for album cards to load
	await page.goto('/');
	await page.waitForLoadState('domcontentloaded');
	await page.locator('[data-testid^="album-card-"]').first().waitFor({ state: 'visible', timeout: 10000 });

	const albumCard = page.locator('[data-testid^="album-card-"]').filter({
		has: page.locator('[data-testid^="album-title-"]').filter({ hasText: new RegExp(`^${albumTitle}$`, 'i') })
	});

	// Check if album exists before trying to click
	const count = await albumCard.count();
	if (count === 0) {
		throw new Error(`Album "${albumTitle}" not found in library. Available albums may not match test data.`);
	}

	// Click the album link (not the play button) to go to details page
	const albumLink = albumCard.locator('a').first();
	await albumLink.click();

	// Wait for album details page
	await page.locator('[data-testid="album-details"]').waitFor({ state: 'visible' });
});

// Multi-browser steps — create a second browser tab named "B" that
// inherits auth from the primary tab and lands on the library home so
// the WS subscription is live before the next step runs.
Given('User has multiple browsers open', async function (this: AudiodWorld) {
	if (this.extraPages.has('B')) return; // idempotent

	const newPage = await this.openBrowser('B', getSharedBrowser(), getActiveDeviceConfig(), getBaseURL());
	await newPage.goto('/');
	await newPage.waitForLoadState('domcontentloaded');
	// Wait for the player footer to be in the DOM — it always renders, but
	// when there's no session it's collapsed (grid-rows-[0fr]). We just
	// need the WS subscription to be wired, which happens on layout mount.
	await newPage.locator('[data-testid="player-footer"]').waitFor({ state: 'attached', timeout: 10000 });
	await waitForClientId(newPage);
});

Given('User has browser on laptop and phone', async function (this: AudiodWorld) {
	// TODO: Create secondary browser context for multi-device testing
	return 'pending';
});

Given("Another user's browser is showing MPD playback", async function (this: AudiodWorld) {
	// TODO: Create secondary user's browser context
});

Given('User has browser that went offline', async function (this: AudiodWorld) {
	// TODO: Simulate browser going offline
	return 'pending';
});

Given('Music is playing in browser A', async function (this: AudiodWorld) {
	// Browser A is the default context — assert it shows an active session.
	const page = this.getBrowser('A');
	await expect(page.locator('[data-testid="player-footer"]')).toBeVisible();

	// Confirm the session is actually playing (not just paused/loaded).
	const response = await page.request.get('/api/playback/session');
	expect(response.ok()).toBe(true);
	const session = await response.json();
	expect(session?.state).toBe('playing');
});

// =====================
// MPD cleanup
// =====================

After(async function (this: AudiodWorld) {
	if (this.mpdDevices.size > 0) {
		// Reset test-mpd state between scenarios
		try {
			await resetTestMpd();
		} catch {
			// Ignore cleanup errors
		}
		// Device records in DB are cleaned by system reset in next scenario's Background
		this.mpdDevices.clear();
	}
});
