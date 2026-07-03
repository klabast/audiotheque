import { beforeEach, describe, expect, it, vi } from 'vitest';

// The playback store is a module-level singleton. Reset modules between
// tests so subscriptions and state are fresh.
beforeEach(() => {
	vi.resetModules();
	vi.clearAllMocks();
});

type WsCb = (data: unknown) => void;

function setupApiMock() {
	const sessionListeners: WsCb[] = [];
	const clientIdListeners: WsCb[] = [];

	vi.doMock('$lib/services/api', () => ({
		api: {
			subscribeToPlaybackSession: vi.fn((cb: WsCb) => {
				sessionListeners.push(cb);
				return () => {};
			}),
			subscribeToClientId: vi.fn((cb: WsCb) => {
				clientIdListeners.push(cb);
				return () => {};
			}),
			getPlaybackSession: vi.fn().mockResolvedValue(null),
			listDevices: vi.fn().mockResolvedValue([]),
			getTrackStreamUrl: vi.fn(() => '/api/tracks/1/stream'),
			playAlbum: vi.fn(),
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

	return { sessionListeners, clientIdListeners };
}

function makeAudioElement() {
	return {
		play: vi.fn().mockResolvedValue(undefined),
		pause: vi.fn(),
		currentTime: 0,
		paused: true,
		volume: 1,
		muted: false
	} as unknown as HTMLAudioElement & {
		play: ReturnType<typeof vi.fn>;
		pause: ReturnType<typeof vi.fn>;
	};
}

describe('playback store — activeDeviceName when remote', () => {
	it('does NOT fall back to "This Device" when remote and the device list is empty', async () => {
		setupApiMock();
		const { playback } = await import('./playback.svelte');

		// Remote MPD session, but devices list hasn't loaded yet (race after refresh).
		playback.updateSession({
			state: 'playing',
			deviceId: 'mpd-living-room',
			current: { trackId: 5, position: 12 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});

		// Sanity: the store must consider this remote.
		expect(playback.isRemoteDevice).toBe(true);
		// The label shown to the user must not say "This Device" — that's the
		// label for local playback. Showing it while remote misleads the user
		// into thinking the browser took over.
		expect(playback.activeDeviceName).not.toBe('This Device');
	});
});

describe('playback store — backend-authoritative (dumb client)', () => {
	// The store is a thin adapter over the API. Command methods MUST NOT poke
	// the audio element — every state change goes through the backend.

	it('play() does not touch the audio element', async () => {
		setupApiMock();
		const { playback } = await import('./playback.svelte');

		const audio = makeAudioElement();
		playback.setAudioElement(audio);
		await playback.loadSession();

		const { api } = await import('$lib/services/api');
		vi.mocked(api.resumePlayback).mockResolvedValue({
			state: 'playing',
			deviceId: '',
			current: { trackId: 5, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		} as never);

		await playback.play();

		expect(audio.play).not.toHaveBeenCalled();
		expect(audio.pause).not.toHaveBeenCalled();
		expect(api.resumePlayback).toHaveBeenCalled();
	});

	it('pause() does not touch the audio element', async () => {
		setupApiMock();
		const { playback } = await import('./playback.svelte');

		const audio = makeAudioElement();
		playback.setAudioElement(audio);
		await playback.loadSession();

		const { api } = await import('$lib/services/api');
		vi.mocked(api.pausePlayback).mockResolvedValue({
			state: 'paused',
			deviceId: '',
			current: { trackId: 5, position: 30 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		} as never);

		await playback.pause();

		expect(audio.pause).not.toHaveBeenCalled();
		expect(api.pausePlayback).toHaveBeenCalled();
	});

	it('seek() goes through the backend even for local playback', async () => {
		setupApiMock();
		const { playback } = await import('./playback.svelte');

		const audio = makeAudioElement();
		playback.setAudioElement(audio);
		await playback.loadSession();

		// Make the audio element believe it's local (currentTime defaults to 0).
		const { api } = await import('$lib/services/api');
		vi.mocked(api.seekPlayback).mockResolvedValue({
			state: 'playing',
			deviceId: '',
			current: { trackId: 5, position: 60 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		} as never);

		await playback.seek(60);

		// Local seek used to bypass the backend by writing audio.currentTime.
		// In the unified model, seek MUST always hit the API so the backend
		// remains the source of truth.
		expect(api.seekPlayback).toHaveBeenCalledWith(60);
	});

	it('setVolume() goes through the backend even for local playback', async () => {
		setupApiMock();
		const { playback } = await import('./playback.svelte');

		const audio = makeAudioElement();
		playback.setAudioElement(audio);
		await playback.loadSession();

		const { api } = await import('$lib/services/api');
		vi.mocked(api.setPlaybackVolume).mockResolvedValue({
			state: 'playing',
			deviceId: '',
			current: { trackId: 5, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] },
			deviceVolumes: { '': 75 }
		} as never);

		// volume is 0..1 in the UI, 0..100 in the backend.
		await playback.setVolume(0.75);

		// Backend must hear about every volume change — otherwise the persisted
		// per-device volume is wrong and the UI value drifts from reality.
		expect(api.setPlaybackVolume).toHaveBeenCalledWith(75);
	});

	it('togglePlayPause() reads session.state, not audioElement.paused', async () => {
		setupApiMock();
		const { playback } = await import('./playback.svelte');

		// Audio element claims it's playing; session says it's paused.
		// togglePlayPause must trust the session (backend is the truth).
		const audio = makeAudioElement();
		(audio as unknown as { paused: boolean }).paused = false;
		playback.setAudioElement(audio);
		await playback.loadSession();

		playback.updateSession({
			state: 'paused',
			deviceId: '',
			current: { trackId: 5, position: 30 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});

		const { api } = await import('$lib/services/api');
		vi.mocked(api.resumePlayback).mockResolvedValue({
			state: 'playing',
			deviceId: '',
			current: { trackId: 5, position: 30 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		} as never);

		await playback.togglePlayPause();

		// Session said paused, so toggle must call resume — regardless of
		// what the audio element thinks its paused state is.
		expect(api.resumePlayback).toHaveBeenCalled();
		expect(api.pausePlayback).not.toHaveBeenCalled();
	});
});

describe('playback store — isLocalAudio requires a real ID match', () => {
	// Under the unified-session invariant the server always names a real
	// playback device. Empty deviceId is no longer a sentinel for "this tab"
	// — local audio is gated on an exact ID match.
	it('isLocalAudio is false when session.deviceId is empty, regardless of thisClientId', async () => {
		const { clientIdListeners } = setupApiMock();
		const { playback } = await import('./playback.svelte');

		await playback.loadSession();
		clientIdListeners.forEach((cb) => cb('tab-A'));

		// Pre-invariant state that survived migration somehow.
		playback.updateSession({
			state: 'playing',
			deviceId: '',
			current: { trackId: 5, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});

		expect(playback.isLocalAudio).toBe(false);
	});

	it('isLocalAudio is true only when session.deviceId === thisClientId', async () => {
		const { clientIdListeners } = setupApiMock();
		const { playback } = await import('./playback.svelte');

		await playback.loadSession();
		clientIdListeners.forEach((cb) => cb('tab-A'));

		playback.updateSession({
			state: 'playing',
			deviceId: 'tab-A',
			current: { trackId: 5, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		});

		expect(playback.isLocalAudio).toBe(true);
	});
});

describe('playback store — onTrackEnded error handling', () => {
	// Regression test for "track ends, player just hangs": if the server
	// rejects/loses the session (e.g. deviceID briefly unresolvable during a
	// WS reconnect), onTrackEnded must not swallow the error silently — it
	// has to refetch the session so the UI reflects what the backend
	// actually has (a resumed session, or none), rather than staying stuck
	// showing the just-ended track as still playing.
	it('refetches the session from the server when next() fails', async () => {
		setupApiMock();
		const { playback } = await import('./playback.svelte');
		await playback.loadSession();

		const { api } = await import('$lib/services/api');
		vi.mocked(api.nextTrack).mockRejectedValue(new Error('no active session'));
		vi.mocked(api.getPlaybackSession).mockResolvedValue({
			state: 'stopped',
			deviceId: '',
			current: null,
			queue: [],
			source: null
		} as never);

		await playback.onTrackEnded();

		expect(api.nextTrack).toHaveBeenCalled();
		expect(api.getPlaybackSession).toHaveBeenCalled();
		expect(playback.isStopped).toBe(true);
	});

	it('does not refetch the session when next() succeeds', async () => {
		setupApiMock();
		const { playback } = await import('./playback.svelte');
		await playback.loadSession();

		const { api } = await import('$lib/services/api');
		vi.mocked(api.nextTrack).mockResolvedValue({
			state: 'playing',
			deviceId: '',
			current: { trackId: 6, position: 0 },
			queue: [],
			source: { type: 'album', id: 100, remaining: [] }
		} as never);

		await playback.onTrackEnded();

		expect(api.nextTrack).toHaveBeenCalled();
		// getPlaybackSession was already called once by loadSession() above;
		// it must not be called again on the happy path.
		expect(api.getPlaybackSession).toHaveBeenCalledTimes(1);
	});
});

describe('playback store — WS-driven device transfers', () => {
	it('pauses local audio when a WS session moves playback AWAY from this tab', async () => {
		const { sessionListeners, clientIdListeners } = setupApiMock();

		const { playback } = await import('./playback.svelte');

		const audio = makeAudioElement();
		playback.setAudioElement(audio);

		// loadSession wires up the WS subscriptions and resolves with no session.
		await playback.loadSession();

		// Hub welcomes this tab as 'tab-A'. Real api.subscribeToClientId
		// unwraps the WS payload and passes the clientId string to subscribers.
		clientIdListeners.forEach((cb) => cb('tab-A'));

		// Tab is currently the active local device, playing a track.
		sessionListeners.forEach((cb) =>
			cb({
				state: 'playing',
				deviceId: 'tab-A',
				current: { trackId: 5, position: 0 },
				queue: [],
				source: { type: 'album', id: 100, remaining: [] }
			})
		);

		// Sanity: this tab is local, audio is not paused yet.
		expect(audio.pause).not.toHaveBeenCalled();

		// Another tab transfers playback to MPD. The server broadcasts the new
		// session — deviceId moves AWAY from this tab.
		sessionListeners.forEach((cb) =>
			cb({
				state: 'playing',
				deviceId: 'mpd-living-room',
				current: { trackId: 5, position: 12 },
				queue: [],
				source: { type: 'album', id: 100, remaining: [] }
			})
		);

		// This tab must pause its <audio> element so it doesn't keep playing
		// alongside MPD.
		expect(audio.pause).toHaveBeenCalled();
	});
});
