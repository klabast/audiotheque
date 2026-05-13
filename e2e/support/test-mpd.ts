import { Page } from '@playwright/test';

const { testConfig } = require('../cucumber');

export interface TestMpdState {
	playState: string;
	currentFile: string;
	elapsed: number;
	volume: number;
	playlist: string[];
}

export interface CommandRecord {
	command: string;
	args?: string;
	timestamp: string;
}

/**
 * Get the current state of test-mpd via its HTTP observation API
 */
export async function getTestMpdState(): Promise<TestMpdState> {
	const response = await fetch(`${testConfig.testMpdURL}/state`);
	if (!response.ok) {
		throw new Error(`test-mpd /state failed: ${response.status}`);
	}
	return response.json();
}

/**
 * Get the command history from test-mpd
 */
export async function getTestMpdHistory(): Promise<CommandRecord[]> {
	const response = await fetch(`${testConfig.testMpdURL}/history`);
	if (!response.ok) {
		throw new Error(`test-mpd /history failed: ${response.status}`);
	}
	return response.json();
}

/**
 * Reset test-mpd state (clear playlist, stop playback, clear history)
 */
export async function resetTestMpd(): Promise<void> {
	const response = await fetch(`${testConfig.testMpdURL}/reset`, { method: 'POST' });
	if (!response.ok) {
		throw new Error(`test-mpd /reset failed: ${response.status}`);
	}
}

/**
 * Simulate the current MPD track finishing — flips state to "stop" so the
 * audiod position poller observes end-of-track and auto-advances.
 */
export async function endCurrentTestMpdTrack(): Promise<void> {
	const response = await fetch(`${testConfig.testMpdURL}/track-ended`, { method: 'POST' });
	if (!response.ok) {
		throw new Error(`test-mpd /track-ended failed: ${response.status}`);
	}
}

/**
 * Disable test-mpd's mixer — subsequent setvol commands return the same
 * "No mixer" ACK that real MPD returns when configured with
 * `mixer_type "none"` (e.g. HiFiBerry's default). Used to validate
 * audiod's tolerance of mixerless devices.
 */
export async function disableTestMpdMixer(): Promise<void> {
	const response = await fetch(`${testConfig.testMpdURL}/mixer-disable`, { method: 'POST' });
	if (!response.ok) {
		throw new Error(`test-mpd /mixer-disable failed: ${response.status}`);
	}
}

/**
 * Register an MPD device via the real settings API.
 * Uses the authenticated page context to call POST /api/settings/devices.
 * Returns the device ID assigned by the server.
 */
export async function createDevice(page: Page, name: string): Promise<string> {
	return createDeviceAt(page, name, testConfig.testMpdAddr);
}

/**
 * Register an MPD device with a custom address. Useful for failure-mode
 * scenarios that need a known-unreachable host:port.
 */
export async function createDeviceAt(page: Page, name: string, address: string): Promise<string> {
	const response = await page.request.post('/api/settings/devices', {
		data: { name, type: 'mpd', address }
	});

	if (!response.ok()) {
		const body = await response.text();
		throw new Error(`Failed to create device "${name}": ${response.status()} ${body}`);
	}

	const device = await response.json();
	return device.ID;
}
