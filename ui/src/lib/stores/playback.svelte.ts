import { api, type DeviceInfo } from '$lib/services/api';
import type { PlaybackSessionResponse } from '$lib/api/client';

/**
 * Playback Store — Backend-Authoritative
 *
 * The BACKEND owns playback state. This store is a thin adapter that:
 *   1. Sends user commands to the backend (HTTP API).
 *   2. Mirrors the backend's session into reactive UI state via REST + WS.
 *
 * It does NOT decide playback behavior or poke the audio element. The
 * <audio> element is reconciled to match `session` by an effect in
 * PlayFooter — that's the only place audio element commands originate, and
 * even there they only EXECUTE the session.
 *
 * Ownership cheat-sheet:
 *   - "What is playing?"   → backend (session)
 *   - "Where is the head?" → backend (session.current.position)
 *   - "How loud?"          → backend (session.deviceVolumes[deviceId])
 *   - "Which device?"      → backend (session.deviceId)
 *   - "Sound waves"        → audio element / MPD (the actual hardware)
 */

export const playback = (() => {
	// Reactive mirror of the backend session.
	let session = $state<PlaybackSessionResponse | null>(null);
	let devices = $state<DeviceInfo[]>([]);
	let thisClientId = $state('');

	// Audio element handle — used only for (a) reading currentTime to push
	// position, (b) being reconciled by the effect in PlayFooter, and (c)
	// pausing on transfer-away to win the unmount race. Command methods on
	// this store NEVER call .play()/.pause()/.currentTime= on it.
	let audioElement = $state<HTMLAudioElement | null>(null);

	// Last non-zero volume for unmute restoration. Pure UI memory — the
	// backend doesn't model "muted vs unmuted", just numeric volume.
	let lastNonZeroVolume = $state(100);

	let wsSubscribed = false;

	// === DERIVED — pure functions of session ===
	const hasSession = $derived(session !== null && session.current !== null);
	const currentTrackId = $derived(session?.current?.trackId ?? null);
	const trackUrl = $derived(currentTrackId ? api.getTrackStreamUrl(currentTrackId) : null);
	const isPlaying = $derived(session?.state === 'playing');
	const isPaused = $derived(session?.state === 'paused');
	const isStopped = $derived(session?.state === 'stopped' || session === null);
	const position = $derived(session?.current?.position ?? 0);

	const deviceId = $derived(session?.deviceId ?? '');
	// "Local audio" means: this tab's <audio> element is the active output.
	// Under the unified-session invariant the server always names a real
	// device — there is no empty-string "play here" sentinel anymore. Local
	// audio requires an explicit ID match.
	const isLocalAudio = $derived(
		thisClientId !== '' && deviceId !== '' && deviceId === thisClientId
	);
	const isRemoteDevice = $derived(!isLocalAudio);
	const activeDevice = $derived(
		isRemoteDevice ? (devices.find((d) => d.id === deviceId) ?? null) : null
	);
	// For LOCAL playback this returns an empty string — the UI is responsible
	// for substituting its localized "This Device" label. Keeping i18n in the
	// frontend means we don't drag a translation table into this store.
	const activeDeviceName = $derived(activeDevice?.name ?? (isRemoteDevice ? 'Remote device' : ''));
	const hasDevices = $derived(devices.length > 0);
	// Volume (0–100): per-device, persisted in session.deviceVolumes.
	const volume100 = $derived(session?.deviceVolumes?.[deviceId] ?? 100);
	const volume = $derived(volume100 / 100); // 0–1 for UI consumers
	// supportsVolume defaults true. The backend only emits
	// deviceCapabilities.volume=false after observing the device reject
	// setvol (mixerless MPD); the UI uses this to grey/lock the slider so
	// users aren't fighting a control that physically can't move.
	const supportsVolume = $derived(session?.deviceCapabilities?.volume !== false);

	return {
		// === REACTIVE GETTERS ===
		get session() {
			return session;
		},
		get hasSession() {
			return hasSession;
		},
		get currentTrackId() {
			return currentTrackId;
		},
		get trackUrl() {
			return trackUrl;
		},
		get isPlaying() {
			return isPlaying;
		},
		get isPaused() {
			return isPaused;
		},
		get isStopped() {
			return isStopped;
		},
		get position() {
			return position;
		},
		get audioElement() {
			return audioElement;
		},
		get devices() {
			return devices;
		},
		get deviceId() {
			return deviceId;
		},
		get isLocalAudio() {
			return isLocalAudio;
		},
		get isRemoteDevice() {
			return isRemoteDevice;
		},
		get activeDeviceName() {
			return activeDeviceName;
		},
		get hasDevices() {
			return hasDevices;
		},
		get volume() {
			return volume;
		},
		get volume100() {
			return volume100;
		},
		get supportsVolume() {
			return supportsVolume;
		},
		get thisClientId() {
			return thisClientId;
		},

		// PlayFooter calls this on mount/unmount of the <audio> element.
		setAudioElement(el: HTMLAudioElement | null) {
			audioElement = el;
		},

		// === WIRING ===

		async loadSession() {
			if (!wsSubscribed) {
				wsSubscribed = true;
				api.subscribeToPlaybackSession((s) => {
					// Pre-update snapshot so we can detect "this tab just lost
					// active-device status" and force a pause — the only
					// audio-element command this layer issues, and only because
					// the {#if} unmount race can leave buffered audio playing
					// after our element leaves the DOM.
					const prevDeviceId = session?.deviceId ?? '';
					const isThisTab = (id: string) => thisClientId !== '' && id !== '' && id === thisClientId;
					const wasLocal = isThisTab(prevDeviceId);
					const newDeviceId = s.deviceId ?? '';
					const isLocalNow = isThisTab(newDeviceId);

					session = s as PlaybackSessionResponse;

					if (wasLocal && !isLocalNow && audioElement) {
						audioElement.pause();
					}
				});
				api.subscribeToClientId((id) => {
					thisClientId = id;
					// The first /api/devices fetch from PlayFooter.onMount ran
					// before this welcome lands, so it had no X-Audiod-Client-Id
					// header and the server couldn't flag the requesting row
					// as `isCurrent`. Re-fetch now so the picker can label the
					// current tab as "This Device". Errors are swallowed —
					// keeping the stale cache is safer than an empty list.
					api
						.listDevices()
						.then((d) => {
							devices = d;
						})
						.catch(() => {});
				});
			}
			try {
				session = ((await api.getPlaybackSession()) as PlaybackSessionResponse) ?? null;
			} catch {
				session = null;
			}
		},

		async loadDevices() {
			try {
				devices = await api.listDevices();
			} catch {
				devices = [];
			}
		},

		// === COMMANDS — pure API calls, never poke the audio element ===

		async playAlbum(albumId: number, startTrackId?: number, targetDeviceId?: string) {
			// "Play album" without an explicit target should continue on the
			// currently-active device (Spotify-style: if MPD is playing, the
			// new album also goes to MPD). If no session exists yet, leave
			// targetDeviceId unset — the server will derive "play here" from
			// X-Audiod-Client-Id.
			const effectiveTarget =
				targetDeviceId ??
				(session?.deviceId !== undefined && session.deviceId !== '' ? session.deviceId : undefined);
			session = (await api.playAlbum(
				albumId,
				startTrackId,
				effectiveTarget
			)) as PlaybackSessionResponse;
		},

		async play() {
			session = (await api.resumePlayback()) as PlaybackSessionResponse;
		},

		async pause() {
			// The audio element is the only thing that knows the precise
			// current head-position when we're playing locally. Read it and
			// pass to the backend so the saved position reflects where the
			// user actually paused. After this call returns, session is the
			// truth and the reconciliation effect handles the audio element.
			const pos = audioElement
				? Math.floor(audioElement.currentTime)
				: (session?.current?.position ?? 0);
			session = (await api.pausePlayback(pos)) as PlaybackSessionResponse;
		},

		async togglePlayPause() {
			// Single source of truth: session.state. If the audio element
			// disagrees with the session, that's a bug to fix elsewhere — not
			// a fork in this method's logic.
			if (session?.state === 'playing') {
				await this.pause();
			} else {
				await this.play();
			}
		},

		async seek(time: number) {
			session = (await api.seekPlayback(time)) as PlaybackSessionResponse;
		},

		async next() {
			session = (await api.nextTrack()) as PlaybackSessionResponse;
		},

		async previous() {
			session = (await api.previousTrack()) as PlaybackSessionResponse;
		},

		async transferPlayback(targetDeviceId: string) {
			session = (await api.transferPlayback(targetDeviceId)) as PlaybackSessionResponse;
		},

		async setVolume(volume0to1: number) {
			const vol = Math.round(Math.max(0, Math.min(1, volume0to1)) * 100);
			if (vol > 0) lastNonZeroVolume = vol;
			session = (await api.setPlaybackVolume(vol)) as PlaybackSessionResponse;
		},

		async toggleMute() {
			// Mute = volume 0; unmute = restore last non-zero volume.
			const target = volume100 > 0 ? 0 : lastNonZeroVolume;
			session = (await api.setPlaybackVolume(target)) as PlaybackSessionResponse;
		},

		async onTrackEnded() {
			// The browser's <audio> 'ended' event is the only signal the
			// backend can't observe directly for local playback. Translate
			// it to a backend "next" command.
			await this.next();
		},

		// Test seam: synchronously inject a session state. Production code
		// gets sessions exclusively from REST/WS responses.
		updateSession(newSession: PlaybackSessionResponse) {
			session = newSession;
		}
	};
})();

export type { PlaybackSessionResponse };
