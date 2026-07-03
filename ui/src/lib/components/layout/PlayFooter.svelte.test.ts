import { beforeEach, describe, expect, it, vi } from 'vitest';
import { render, screen, fireEvent } from '@testing-library/svelte';
import { tick } from 'svelte';
import '@testing-library/jest-dom/vitest';
import PlayFooter from './PlayFooter.svelte';
import { playback } from '$lib/stores/playback.svelte';
import { api } from '$lib/services/api';

// Captured clientId listeners so tests can simulate the WS welcome that
// assigns this tab its hub client ID. "Local audio" depends on a real ID
// match (no empty-string sentinel).
const clientIdListeners: Array<(id: string) => void> = [];

vi.mock('$lib/services/api', () => ({
	api: {
		getAlbum: vi.fn().mockResolvedValue({ id: 1, title: 'A', artistName: 'X' }),
		listAlbumTracks: vi.fn().mockResolvedValue([]),
		playAlbum: vi.fn(),
		getPlaybackSession: vi.fn().mockResolvedValue(null),
		subscribeToPlaybackSession: vi.fn(() => () => {}),
		subscribeToClientId: vi.fn((cb: (id: string) => void) => {
			clientIdListeners.push(cb);
			return () => {};
		}),
		listDevices: vi.fn().mockResolvedValue([]),
		getTrackStreamUrl: vi.fn(() => '/api/tracks/1/stream'),
		pausePlayback: vi.fn(),
		resumePlayback: vi.fn(),
		nextTrack: vi.fn(),
		previousTrack: vi.fn(),
		seekPlayback: vi.fn(),
		setPlaybackVolume: vi.fn(),
		transferPlayback: vi.fn(),
		sendPlaybackPosition: vi.fn()
	}
}));

beforeEach(() => {
	vi.clearAllMocks();
	clientIdListeners.length = 0;
});

/** Simulate the WS welcome that gives this tab its hub-assigned client ID. */
function assignClientId(id: string) {
	clientIdListeners.forEach((cb) => cb(id));
}

describe('PlayFooter — reconciliation: session drives audio element', () => {
	it('pauses the <audio> element when session.state flips to paused', async () => {
		render(PlayFooter);
		await new Promise((r) => setTimeout(r, 0));

		// Hub welcome — this tab is 'tab-A'. The session must address us by
		// that ID for isLocalAudio (and thus the <audio> element) to render.
		assignClientId('tab-A');
		playback.updateSession({
			state: 'playing',
			deviceId: 'tab-A',
			current: { trackId: 5, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});
		await tick();

		const audio = screen.getByTestId('audio-element') as HTMLAudioElement;
		// Force the element into "not paused" so we can detect a reconcile pause.
		// jsdom's HTMLMediaElement is barely real — overriding `paused` is enough
		// to trick the effect's "if (!audioRef.paused) pause()" branch.
		Object.defineProperty(audio, 'paused', { configurable: true, value: false });
		const pauseSpy = vi.spyOn(audio, 'pause');

		// Backend says "now paused" — reconciliation must call audio.pause().
		playback.updateSession({
			state: 'paused',
			deviceId: 'tab-A',
			current: { trackId: 5, position: 30 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});
		await tick();

		expect(pauseSpy).toHaveBeenCalled();
	});
});

describe('PlayFooter — local audio element rendering', () => {
	it('renders without hanging', async () => {
		render(PlayFooter);
		await tick();
		await tick();
		await new Promise((r) => setTimeout(r, 50));
		// Just verify the footer rendered.
		expect(screen.queryByTestId('player-footer')).toBeInTheDocument();
	});

	it('does NOT render the <audio> element when playback is on a remote device', async () => {
		render(PlayFooter);
		// onMount kicks off loadSession() which resolves to null with the mock;
		// wait for it to settle so the subsequent updateSession isn't clobbered.
		await new Promise((r) => setTimeout(r, 0));

		playback.updateSession({
			state: 'playing',
			deviceId: 'mpd-living-room',
			current: { trackId: 5, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});
		await tick();

		expect(screen.queryByTestId('audio-element')).not.toBeInTheDocument();
	});

	it('DOES render the <audio> element when playback is local', async () => {
		render(PlayFooter);
		await new Promise((r) => setTimeout(r, 0));

		assignClientId('tab-A');
		playback.updateSession({
			state: 'playing',
			deviceId: 'tab-A',
			current: { trackId: 5, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});
		await tick();

		expect(screen.queryByTestId('audio-element')).toBeInTheDocument();
	});
});

describe('PlayFooter — ended watchdog', () => {
	// Regression coverage for "track ends, player just hangs": the browser's
	// `onended` is the only advance trigger, and it can fail to fire (or fail
	// to advance) — timeupdate keeps ticking, so it's used as a second look:
	// if the element reports ended but the session still says playing, force
	// the advance.
	it('forces an advance when the audio element reports ended but the session still says playing', async () => {
		render(PlayFooter);
		await new Promise((r) => setTimeout(r, 0));

		assignClientId('tab-A');
		playback.updateSession({
			state: 'playing',
			deviceId: 'tab-A',
			current: { trackId: 5, position: 179 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});
		await tick();

		const audio = screen.getByTestId('audio-element') as HTMLAudioElement;
		Object.defineProperty(audio, 'ended', { configurable: true, value: true });

		vi.mocked(api.nextTrack).mockResolvedValue({
			state: 'playing',
			deviceId: 'tab-A',
			current: { trackId: 6, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		} as never);

		await fireEvent.timeUpdate(audio);
		// Let the onTrackEnded() -> next() async chain settle.
		await new Promise((r) => setTimeout(r, 0));

		expect(api.nextTrack).toHaveBeenCalledTimes(1);
	});

	it('fires at most once per track even across repeated timeupdate ticks', async () => {
		render(PlayFooter);
		await new Promise((r) => setTimeout(r, 0));

		assignClientId('tab-A');
		playback.updateSession({
			state: 'playing',
			deviceId: 'tab-A',
			current: { trackId: 5, position: 179 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});
		await tick();

		const audio = screen.getByTestId('audio-element') as HTMLAudioElement;
		Object.defineProperty(audio, 'ended', { configurable: true, value: true });

		// next() fails, so the session (and its trackId) never changes — proves
		// the per-track guard, not a side-effect of a new session, prevents
		// the second firing.
		vi.mocked(api.nextTrack).mockRejectedValue(new Error('no active session'));
		vi.mocked(api.getPlaybackSession).mockResolvedValue({
			state: 'playing',
			deviceId: 'tab-A',
			current: { trackId: 5, position: 179 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		} as never);

		await fireEvent.timeUpdate(audio);
		await new Promise((r) => setTimeout(r, 0));
		await fireEvent.timeUpdate(audio);
		await new Promise((r) => setTimeout(r, 0));

		expect(api.nextTrack).toHaveBeenCalledTimes(1);
	});
});
