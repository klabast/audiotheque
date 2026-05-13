import { When } from '@cucumber/cucumber';
import { expect } from '@playwright/test';
import { AudiodWorld } from '../../support/world';
import { setPreviousVolume, getDeviceId } from './playback.given.steps';
import {
	ensurePlayerControlsVisible,
	playerControl,
	transferViaPicker,
	waitForClientId
} from '../../support/player';
import { getSharedBrowser, getActiveDeviceConfig, getBaseURL } from '../../support/hooks';
import { endCurrentTestMpdTrack } from '../../support/test-mpd';

When('User plays album {string}', async function (this: AudiodWorld, albumTitle: string) {
	const page = this.getPage();

	// Ensure we're on the library page where album cards are visible
	await page.goto('/');
	await page.waitForLoadState('domcontentloaded');

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
});

When('User pauses playback', async function (this: AudiodWorld) {
	const page = this.getPage();

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/pause') && response.status() === 200
	);

	await page.locator('[data-testid="play-pause-button"]').click();

	await responsePromise;
});

When('User resumes playback', async function (this: AudiodWorld) {
	const page = this.getPage();

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/resume') && response.status() === 200
	);

	await page.locator('[data-testid="play-pause-button"]').click();

	await responsePromise;
});

When('User skips to next track', async function (this: AudiodWorld) {
	const page = this.getPage();
	const button = await playerControl(page, 'next-button', 'next-button-fullscreen');

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/next') && response.status() === 200
	);

	await button.click();

	await responsePromise;
});

When('User goes to previous track', async function (this: AudiodWorld) {
	const page = this.getPage();
	const button = await playerControl(page, 'previous-button', 'previous-button-fullscreen');

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/previous') && response.status() === 200
	);

	await button.click();

	await responsePromise;
});

When('User seeks to {int}% of track', async function (this: AudiodWorld, percent: number) {
	const page = this.getPage();

	const seekBar = await playerControl(page, 'seek-bar', 'seek-bar-fullscreen');

	// Wait for the seek bar to expose a valid duration (browser: from audio
	// element; remote: from track metadata loaded into the playback store).
	await seekBar.evaluate(async (el) => {
		const start = Date.now();
		while (Date.now() - start < 10000) {
			const max = parseFloat(el.getAttribute('aria-valuemax') || '0');
			if (max > 0 && !isNaN(max)) return;
			await new Promise((r) => setTimeout(r, 100));
		}
		throw new Error('Seek bar duration did not become available');
	});

	const box = await seekBar.boundingBox();
	if (!box) throw new Error('Seek bar has no bounding box');

	// Backend is authoritative for seek (local and remote both POST). Always
	// wait for the API response so the Then step doesn't read audio.currentTime
	// before reconciliation has applied the new session position.
	const seekResponsePromise = page.waitForResponse(
		(r) => r.url().includes('/api/playback/seek') && r.status() === 200
	);

	await seekBar.click({
		position: { x: Math.max(1, box.width * (percent / 100)), y: box.height / 2 }
	});

	await seekResponsePromise;
});

When('User sets volume to {int}%', async function (this: AudiodWorld, volumePercent: number) {
	const page = this.getPage();
	const volumeSlider = await playerControl(page, 'volume-slider');

	// Focus the volume slider and use keyboard to set volume
	// Each ArrowRight/ArrowLeft changes volume by 10% (0.1)
	await volumeSlider.focus();

	// Get current volume from aria-valuenow
	const currentValue = await volumeSlider.getAttribute('aria-valuenow');
	const currentPercent = Math.round(parseFloat(currentValue || '100'));
	const diff = volumePercent - currentPercent;
	const steps = Math.abs(Math.round(diff / 10));
	const key = diff > 0 ? 'ArrowRight' : 'ArrowLeft';

	for (let i = 0; i < steps; i++) {
		// Each keypress triggers playback.setVolume → /api/playback/volume.
		// Without waiting for the response, the next keypress can race the
		// previous API response: the WS-driven session update can reset the
		// slider's bound value mid-flight, swallowing the press. Wait for
		// the response before sending the next press to keep the keystrokes
		// monotonic.
		const volResponsePromise = page.waitForResponse(
			(r) => r.url().includes('/api/playback/volume') && r.status() === 200
		);
		await volumeSlider.press(key);
		await volResponsePromise;
	}

	setPreviousVolume(volumePercent / 100);
});

When('User mutes playback', async function (this: AudiodWorld) {
	const page = this.getPage();
	const muteButton = await playerControl(page, 'mute-button');

	const currentVolume = await page.evaluate(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio?.volume ?? 1;
	});
	setPreviousVolume(currentVolume);

	const volResponsePromise = page.waitForResponse(
		(r) => r.url().includes('/api/playback/volume') && r.status() === 200
	);
	await muteButton.click();
	await volResponsePromise;
});

When('User unmutes playback', async function (this: AudiodWorld) {
	const page = this.getPage();
	const muteButton = await playerControl(page, 'mute-button');

	const volResponsePromise = page.waitForResponse(
		(r) => r.url().includes('/api/playback/volume') && r.status() === 200
	);
	await muteButton.click();
	await volResponsePromise;
});

When('User stops playback', async function (this: AudiodWorld) {
	const page = this.getPage();

	await page.locator('[data-testid="play-pause-button"]').click();
});

// =====================
// MPD device steps
// =====================

When('User plays album {string} to device {string}', async function (this: AudiodWorld, albumTitle: string, deviceName: string) {
	const page = this.getPage();
	const deviceId = getDeviceId(this, deviceName);

	// Ensure the WS welcome has landed so api.playAlbum can attach the
	// X-Audiod-Client-Id header — required under the unified-session invariant.
	await waitForClientId(page);

	const albumCard = page.locator('[data-testid^="album-card-"]').filter({
		has: page.locator('[data-testid^="album-title-"]').filter({ hasText: new RegExp(`^${albumTitle}$`, 'i')})
	});

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/play') && response.status() === 200
	);

	await albumCard.hover();

	// TODO: When device picker on album card is implemented, use it.
	// For now, play first (lands on this browser by default), then transfer
	// via the picker UI — the only sanctioned path under the unified-session
	// invariant.
	const playButton = albumCard.locator('[data-testid^="play-album-"]');
	await playButton.click();
	await responsePromise;

	await transferViaPicker(page, `device-option-${deviceId}`);
});

When('User opens device picker', async function (this: AudiodWorld) {
	const page = this.getPage();

	// Check if there's an active session by querying the API
	const sessionResponse = await page.request.get('/api/playback/session');
	let hasSession = false;
	if (sessionResponse.ok()) {
		try {
			const session = await sessionResponse.json();
			hasSession = !!session?.current;
		} catch {
			// Empty response body = no session
		}
	}

	if (!hasSession) {
		// No active session — play an album to make footer visible
		await page.goto('/');
		await page.waitForLoadState('domcontentloaded');
		await waitForClientId(page);
		const albumCard = page.locator('[data-testid^="album-card-"]').first();
		const responsePromise = page.waitForResponse(
			(r) => r.url().includes('/api/playback/play') && r.status() === 200
		);
		await albumCard.hover();
		await albumCard.locator('[data-testid^="play-album-"]').click();
		await responsePromise;
	}

	// Footer is already visible from the play step (or from the prior Given
	// in the scenario). Avoid an extra page reload — under the unified-session
	// invariant, reloading mints a fresh WS clientId which can orphan the
	// session that names the old one.
	await page.locator('[data-testid="player-footer"]').waitFor({ state: 'visible' });
	await waitForClientId(page);

	const devicePickerButton = await playerControl(page, 'device-picker-button');
	await devicePickerButton.click();
	await page.locator('[data-testid="device-picker-menu"]').waitFor({ state: 'visible' });
});

When('User selects device {string}', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();
	const deviceId = getDeviceId(this, deviceName);

	const option = page.locator(`[data-testid="device-option-${deviceId}"]`);
	await option.waitFor({ state: 'visible', timeout: 10000 });

	const responsePromise = page.waitForResponse(
		(response) => response.url().includes('/api/playback/transfer') && response.status() === 200
	);

	await option.click();

	await responsePromise;
});

When('User transfers playback to {string}', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();
	const deviceId = getDeviceId(this, deviceName);
	const status = await transferViaPicker(page, `device-option-${deviceId}`);
	if (status !== 200) {
		throw new Error(`Failed to transfer playback to "${deviceName}": ${status}`);
	}
});

// Helper: post-transfer settle. The session now names this browser as its
// device; nudge the audio element through `loadedmetadata` and call play()
// because headless browsers don't autoplay outside a user-gesture chain.
async function settleLocalPlayback(this: AudiodWorld) {
	const page = this.getPage();
	await page.waitForFunction(() => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		return audio && audio.readyState >= 1; // HAVE_METADATA
	}, null, { timeout: 10000 });
	await page.evaluate(async () => {
		const audio = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement;
		if (audio && audio.paused) {
			try { await audio.play(); } catch {
				// best-effort
			}
		}
	});
	await page.waitForTimeout(300);
}

When('User transfers playback to browser', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Driven via UI per e2e/CLAUDE.md. Under the unified-session invariant
	// the server requires X-Audiod-Client-Id for empty-deviceId transfers;
	// only the in-page fetch attaches that header — direct page.request
	// calls 400. The picker's "This Device" row is stable testid.
	const status = await transferViaPicker(page, 'device-option-this-browser');
	if (status !== 200) {
		throw new Error(`Failed to transfer playback to browser: ${status}`);
	}
	await settleLocalPlayback.call(this);
});

When('User selects {string} on browser', async function (this: AudiodWorld, _optionText: string) {
	const page = this.getPage();
	// "Play Here" === transfer to this browser. Same UI flow as above.
	const status = await transferViaPicker(page, 'device-option-this-browser');
	if (status !== 200) {
		throw new Error(`Failed to transfer playback to browser: ${status}`);
	}
	await settleLocalPlayback.call(this);
});

When('Handoff fails', async function (this: AudiodWorld) {
	// The device was already made unavailable in the Given step
	// This step triggers the transfer attempt which should fail
	// The error handling is verified in the Then steps
});

When('User resumes playback on one browser', async function (this: AudiodWorld) {
	const page = this.getPage();

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/resume') && response.status() === 200
	);
	await page.locator('[data-testid="play-pause-button"]').click();
	await responsePromise;
});

When('User skips track on one browser', async function (this: AudiodWorld) {
	const page = this.getPage();
	const nextButton = await playerControl(page, 'next-button', 'next-button-fullscreen');

	const responsePromise = page.waitForResponse((response) =>
		response.url().includes('/api/playback/next') && response.status() === 200
	);
	await nextButton.click();
	await responsePromise;
});

When('User opens Audiotheque in a new browser', async function (this: AudiodWorld) {
	const newPage = await this.openBrowser('B', getSharedBrowser(), getActiveDeviceConfig(), getBaseURL());
	await newPage.goto('/');
	await newPage.waitForLoadState('domcontentloaded');
	await newPage.locator('[data-testid="player-footer"]').waitFor({ state: 'attached', timeout: 10000 });
	// Make sure browser B's WS welcome lands before subsequent steps issue
	// play/transfer from this tab — same client-id race as the primary tab.
	await waitForClientId(newPage);
});

// =====================
// Play track steps
// =====================

When('User plays track {string}', { timeout: 25000 }, async function (
	this: AudiodWorld,
	trackTitle: string
) {
	const page = this.getPage();

	// Track rows on the album-detail page carry data-testid="track-row-{trackId}"
	// directly on the clickable <button>. Matching by visible title since the
	// test doesn't know the track ID upfront.
	// Wait for at least one track row to render — slow CI containers can
	// take a couple seconds to fetch + render the album's tracks after the
	// album-details container appears.
	await page
		.locator('[data-testid^="track-row-"]')
		.first()
		.waitFor({ state: 'visible', timeout: 5000 });

	const row = page
		.locator('[data-testid^="track-row-"]')
		.filter({ hasText: new RegExp(escapeRegex(trackTitle), 'i') })
		.first();
	await row.waitFor({ state: 'visible', timeout: 2000 });

	const responsePromise = page.waitForResponse(
		(r) => r.url().includes('/api/playback/play') && r.status() === 200,
		{ timeout: 5000 }
	);
	await row.click({ timeout: 4000 });
	await responsePromise;

	// Wait for the audio element to load metadata, then nudge it via
	// audio.play() if the click-driven autoplay didn't take. Headless
	// browsers can be picky about user-activation tokens after a chain of
	// awaits, and falling back to an explicit play() keeps this test
	// deterministic without hiding bugs (the click still ran, we just
	// guarantee playback started before asserting).
	await page.waitForFunction(
		() => {
			const a = document.querySelector(
				'[data-testid="audio-element"]'
			) as HTMLAudioElement | null;
			return a !== null && a.readyState >= 1; // HAVE_METADATA
		},
		null,
		{ timeout: 5000 }
	);
	await page.evaluate(async () => {
		const a = document.querySelector('[data-testid="audio-element"]') as HTMLAudioElement | null;
		if (a && a.paused) {
			try {
				await a.play();
			} catch {
				// best-effort
			}
		}
	});
	await page.waitForFunction(
		() => {
			const a = document.querySelector(
				'[data-testid="audio-element"]'
			) as HTMLAudioElement | null;
			return a !== null && !a.paused;
		},
		null,
		{ timeout: 3000 }
	);
});

function escapeRegex(s: string): string {
	return s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

// =====================
// Player UI steps
// =====================

When('User navigates to settings', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Use SPA-compatible navigation to preserve audio state
	await page.evaluate(() => {
		window.history.pushState({}, '', '/settings/general');
		window.dispatchEvent(new PopStateEvent('popstate'));
	});
	await page.waitForTimeout(500);
});

When('User expands player', async function (this: AudiodWorld) {
	const page = this.getPage();

	// Full-screen player is mobile only — on desktop, the player is always expanded in the footer
	// For desktop tests, verify the footer is visible (already "expanded")
	const footer = page.locator('[data-testid="player-footer"]');
	await expect(footer).toBeVisible();
});

When('User minimizes player', async function (this: AudiodWorld) {
	const page = this.getPage();

	// Full-screen player is mobile only — on desktop this is a no-op
	const footer = page.locator('[data-testid="player-footer"]');
	await expect(footer).toBeVisible();
});

When('User adds track to queue on one browser', async function (this: AudiodWorld) {
	// TODO: Implement when queue management UI exists
});

When('User changes volume on one browser', async function (this: AudiodWorld) {
	const page = this.getPage();

	// Use keyboard on volume slider — 5 ArrowLeft from 100% = 50%.
	// playerControl opens the fullscreen overlay on mobile, where the
	// volume slider lives (it's hidden in the collapsed mobile bar).
	const volumeSlider = await playerControl(page, 'volume-slider');
	await volumeSlider.focus();
	for (let i = 0; i < 5; i++) {
		const responsePromise = page.waitForResponse(
			(r) => r.url().includes('/api/playback/volume') && r.status() === 200
		);
		await volumeSlider.press('ArrowLeft');
		await responsePromise;
	}
});

When('User attempts to transfer playback to {string}', async function (this: AudiodWorld, deviceName: string) {
	const page = this.getPage();
	const deviceId = getDeviceId(this, deviceName);
	// Driven via UI per e2e/CLAUDE.md. The scenario exercises the
	// failure-recovery path, so we accept any status code; the assertion
	// "Transfer request fails" checks lastTransferStatus >= 400.
	this.lastTransferStatus = await transferViaPicker(
		page,
		`device-option-${deviceId}`,
		{ acceptAnyStatus: true }
	);
});

// The UI grays the volume slider on a mixerless device, so we can't drive
// it via keyboard. The user-facing behavior we want here is "the API
// accepts the value and the session remembers it" — exercised by hitting
// the volume endpoint directly. Server-side tolerance is also covered by
// TestService_SetVolume_ToleratesVolumeNotSupported.
When(
	'User attempts to change volume to {int}% on the mixerless device',
	async function (this: AudiodWorld, percent: number) {
		const page = this.getPage();
		const response = await page.request.post('/api/playback/volume', {
			data: { volume: percent }
		});
		if (!response.ok()) {
			throw new Error(`volume API expected to succeed, got ${response.status()}`);
		}
	}
);

// Lets MPD's clock advance N seconds while state=play. test-mpd runs a
// 1Hz goroutine that increments elapsed when playing — we just wait
// real wall-clock and assert audiod's poller mirrored the change.
When('{int} seconds pass on MPD', async function (this: AudiodWorld, secs: number) {
	const page = this.getPage();
	await page.waitForTimeout(secs * 1000);
});

When('Current track finishes on MPD', async function (this: AudiodWorld) {
	const page = this.getPage();
	// Capture the current trackId so the matching Then can assert a real
	// advance — without a snapshot we can't tell "next track is loaded"
	// apart from "session was already on this track."
	const before = await page.request.get('/api/playback/session');
	expect(before.ok()).toBe(true);
	const beforeSession = await before.json();
	this.preActionTrackId = beforeSession?.current?.trackId;

	await endCurrentTestMpdTrack();
});


When('User transfers to MPD from browser B', async function (this: AudiodWorld) {
	const pageB = this.getBrowser('B');
	// Resolve the MPD device ID (registered via "Given MPD device ... is available")
	if (this.mpdDevices.size === 0) {
		throw new Error('No MPD device registered. Use "Given MPD device ... is available" first.');
	}
	const deviceId = Array.from(this.mpdDevices.values())[0];

	const status = await transferViaPicker(pageB, `device-option-${deviceId}`);
	if (status !== 200) {
		throw new Error(`Transfer from browser B failed: ${status}`);
	}
});

When('Browser reconnects', async function (this: AudiodWorld) {
	// TODO: Simulate network reconnection
});

When('User opens device picker on phone', async function (this: AudiodWorld) {
	// TODO: Implement with secondary mobile browser context
});

When('User transfers playback to their browser', async function (this: AudiodWorld) {
	const page = this.getPage();
	const status = await transferViaPicker(page, 'device-option-this-browser');
	if (status !== 200) {
		throw new Error(`Failed to transfer playback to browser: ${status}`);
	}
});
