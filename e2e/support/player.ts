import type { Locator, Page } from '@playwright/test';

export async function isMobileViewport(page: Page): Promise<boolean> {
	return page.evaluate(() => window.innerWidth < 640);
}

// Wait until the WebSocket welcome lands and the playback store records this
// tab's server-assigned client ID — exposed on <body data-audiod-client-id="…">
// by PlayFooter. Under the unified-session invariant, POST /api/playback/play
// and /transfer with no explicit deviceId fall back to the X-Audiod-Client-Id
// header; the UI only attaches that header once the WS welcome arrives. So a
// play-click that races the welcome will 400 and the waitForResponse(200)
// helpers in steps will hang. Call this after page.goto('/') in any Given
// step that drives playback.
export async function waitForClientId(page: Page, timeoutMs = 10000): Promise<string> {
	const handle = await page.waitForFunction(
		() => document.body.dataset.audiodClientId || null,
		null,
		{ timeout: timeoutMs }
	);
	const value = await handle.jsonValue();
	if (typeof value !== 'string' || !value) {
		throw new Error('WS client id never appeared on <body data-audiod-client-id>');
	}
	return value;
}

// Reads the X-Audiod-Client-Id this tab will send on requests. Tests use this
// to assert "session.deviceId === this browser's id" — replaces the old
// pre-invariant assertion that the deviceId was empty when local.
export async function getClientId(page: Page): Promise<string> {
	const value = await page.evaluate(() => document.body.dataset.audiodClientId || '');
	return value;
}

// On mobile, the collapsed PlayFooter only exposes play/pause. All other
// controls (next, previous, seek, volume, mute, device picker) live in the
// full-screen player, opened by tapping the bar.
export async function ensurePlayerControlsVisible(page: Page): Promise<void> {
	if (!(await isMobileViewport(page))) return;

	const sentinel = page.locator('[data-testid="play-pause-button-fullscreen"]');
	if (await sentinel.isVisible().catch(() => false)) return;

	await page.locator('[data-testid="player-track-info"]').click();
	await sentinel.waitFor({ state: 'visible' });
}

// Resolves a player-control locator. On mobile, opens the full-screen player
// first and scopes the lookup to its overlay so testids that exist in both
// the (hidden) desktop bar and the mobile overlay don't collide.
// `mobileTestid` defaults to `desktopTestid` for controls that share a name.
export async function playerControl(
	page: Page,
	desktopTestid: string,
	mobileTestid: string = desktopTestid
): Promise<Locator> {
	await ensurePlayerControlsVisible(page);
	if (await isMobileViewport(page)) {
		return page
			.locator('[data-testid="player-fullscreen"]')
			.locator(`[data-testid="${mobileTestid}"]`);
	}
	return page.locator(`[data-testid="${desktopTestid}"]`);
}

// Opens the device picker and clicks the option matching `targetTestid`.
// Returns the captured /api/playback/transfer response so callers can read
// the status for failure-mode scenarios. Use 'device-option-this-browser'
// for "transfer to me", or `device-option-${mpdId}` for a named device.
//
// Under the unified-session invariant the server requires X-Audiod-Client-Id
// for empty-deviceId transfers — going through the UI (which sets the
// header via api.transferPlayback) is the only sanctioned path. Direct
// page.request.post(...) with `deviceId: ''` 400s.
export async function transferViaPicker(
	page: Page,
	targetTestid: string,
	{ acceptAnyStatus = false }: { acceptAnyStatus?: boolean } = {}
): Promise<number> {
	await page.locator('[data-testid="player-footer"]').waitFor({ state: 'visible' });
	const pickerButton = await playerControl(page, 'device-picker-button');
	await pickerButton.click();
	const menu = page.locator('[data-testid="device-picker-menu"]');
	await menu.waitFor({ state: 'visible' });

	const responsePromise = page.waitForResponse(
		(r) =>
			r.url().includes('/api/playback/transfer') &&
			(acceptAnyStatus || r.status() === 200)
	);
	await menu.locator(`[data-testid="${targetTestid}"]`).click();
	const response = await responsePromise;
	return response.status();
}
