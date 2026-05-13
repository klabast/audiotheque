<script lang="ts">
	import { playback } from '$lib/stores/playback.svelte';
	import { onMount } from 'svelte';
	import * as m from '$lib/paraglide/messages';
	import { AudPlayFooter } from '$lib/components/ui';
	import { api } from '$lib/services/api';
	import type { LibraryAlbumResponse, LibraryTrackResponse } from '$lib/api/client';

	// === AUDIO ELEMENT ===
	// Mounts/unmounts with the {#if !isRemote} guard, so it's reactive.
	let audioRef = $state<HTMLAudioElement | undefined>(undefined);

	// Read-only mirrors of the audio element's state — populated by bind:
	// directives. They're used ONLY for display (smooth time/duration
	// updates). The store/session is the source of truth for state
	// transitions; the reconciliation effect below makes the audio element
	// match the session.
	let localCurrentTime = $state(0);
	let localDuration = $state(0);

	// === UI STATE ===
	let isFullScreenOpen = $state(false);

	// === SESSION-DERIVED DISPLAY VALUES ===
	const hasSession = $derived(playback.hasSession);
	const trackUrl = $derived(playback.trackUrl);
	const isRemote = $derived(playback.isRemoteDevice);
	const isLocal = $derived(playback.isLocalAudio);
	// "Paused" comes from the session — single source of truth. The audio
	// element will be reconciled to match.
	const paused = $derived(!playback.isPlaying);
	const volume = $derived(playback.volume); // 0..1
	const muted = $derived(playback.volume === 0);
	const supportsVolume = $derived(playback.supportsVolume);

	// Mirror the WS-issued client ID onto <body data-audiod-client-id> so E2E
	// tests can wait for the WS welcome before clicking play/transfer. Under
	// the unified-session invariant the server REQUIRES X-Audiod-Client-Id when
	// no explicit deviceId is sent; clicking before the welcome lands would
	// 400. The dataset write is a no-op during SSR — guard for it explicitly.
	$effect(() => {
		if (typeof document === 'undefined') return;
		const id = playback.thisClientId;
		if (id) {
			document.body.dataset.audiodClientId = id;
		} else {
			delete document.body.dataset.audiodClientId;
		}
	});

	// Album metadata cache (keyed by album ID to avoid refetching on next/previous)
	let cachedAlbumId = $state<number | null>(null);
	let cachedAlbum = $state<LibraryAlbumResponse | null>(null);
	let cachedTracks = $state<LibraryTrackResponse[]>([]);

	// Derive album ID from session source
	const albumId = $derived(
		playback.session?.source?.type === 'album' ? (playback.session.source.id ?? null) : null
	);
	const currentTrackId = $derived(playback.session?.current?.trackId ?? null);

	// Fetch album metadata when album ID changes
	$effect(() => {
		const id = albumId;
		if (id === null) {
			cachedAlbumId = null;
			cachedAlbum = null;
			cachedTracks = [];
			return;
		}
		if (id === cachedAlbumId) return;

		const fetchId = id;
		Promise.all([api.getAlbum(fetchId), api.listAlbumTracks(fetchId)])
			.then(([album, tracks]) => {
				if (albumId === fetchId) {
					cachedAlbumId = fetchId;
					cachedAlbum = album;
					cachedTracks = tracks;
				}
			})
			.catch(() => {
				// Silently fail — keep showing fallback values.
			});
	});

	const matchedTrack = $derived(
		currentTrackId !== null ? (cachedTracks.find((t) => t.id === currentTrackId) ?? null) : null
	);

	// Duration: audio element when local (sub-second granularity), track
	// metadata (ms → s) when remote. Unified to seconds for the seek bar.
	const duration = $derived(
		isLocal && localDuration > 0 ? localDuration : (matchedTrack?.duration ?? 0) / 1000
	);

	// currentTime: audio element when local (smooth), session.position when remote.
	// Server pushes session.position every ~1s for MPD; that's our display rate.
	const currentTime = $derived(isLocal ? localCurrentTime : playback.position);

	const trackTitle = $derived(
		matchedTrack?.title ?? (currentTrackId !== null ? `Track ${currentTrackId}` : '--')
	);
	const trackArtist = $derived(matchedTrack?.artistName ?? cachedAlbum?.artistName ?? '--');
	const trackAlbum = $derived(cachedAlbum?.title ?? '');
	const albumCover = $derived<string | null>(albumId ? `/api/albums/${albumId}/cover` : null);

	// activeDeviceName is empty for local playback — substitute the localized
	// "This Device" label here, where paraglide is in scope.
	const deviceName = $derived(
		playback.isLocalAudio ? m['playback.this_device']() : playback.activeDeviceName
	);
	const isRemoteDevice = $derived(playback.isRemoteDevice);
	const showDeviceSelector = $derived(playback.hasDevices);
	const devices = $derived(playback.devices);
	const currentDeviceId = $derived(playback.deviceId);

	// === BACKEND POSITION REPORTING (browser-as-device only) ===
	// The audio element's clock is the only thing the backend can't observe
	// for local playback. Push it back so the saved position stays current
	// and other clients see live progress.
	const POSITION_SYNC_INTERVAL_MS = 500;
	let lastPositionSent = 0;
	function handleTimeUpdate() {
		if (!isLocal) return;
		if (audioRef?.paused) return;
		const now = Date.now();
		if (now - lastPositionSent < POSITION_SYNC_INTERVAL_MS) return;
		// Don't push position before the reconciliation effect has had a
		// chance to seek to the saved position — otherwise we'd push 0 and
		// overwrite the real position on the server.
		if (!metadataReady) return;
		// Skip while catching up to a server-driven seek: if the audio's
		// local time is far from the session's authoritative position, the
		// reconciliation effect hasn't applied the seek yet. Broadcasting
		// our stale localCurrentTime would clobber the server's correct
		// value (e.g. transfer-after-seek would lose the 45s seek and
		// resume playback at 0). Once reconciliation lands, the gap closes
		// and broadcasts resume.
		const sessionPos = playback.session?.current?.position ?? 0;
		if (Math.abs(localCurrentTime - sessionPos) > 2) return;
		api.sendPlaybackPosition(localCurrentTime);
		lastPositionSent = now;
	}

	// Best-effort final position push when the page is about to disappear.
	function pushPositionNow() {
		if (!isLocal || audioRef?.paused || !playback.session?.current) return;
		api.sendPlaybackPosition(localCurrentTime);
	}

	// === RECONCILIATION ===
	// "metadataReady" gates seeks: HTMLMediaElement.currentTime= throws
	// before metadata loads. Reset it on every load, set when metadata arrives.
	let metadataReady = $state(false);
	function handleLoadStart() {
		metadataReady = false;
	}
	function handleLoadedMetadata() {
		metadataReady = true;
		// Reconciliation effect below will pick this up and seek if needed.
	}

	// THE reconciliation effect: drive the audio element to match the session.
	// This is the ONLY place that calls audio.play()/pause()/currentTime=
	// when responding to session state changes. Command methods on the store
	// don't poke the audio element directly — they call the backend, the
	// backend updates session, this effect makes the speaker comply.
	$effect(() => {
		if (!audioRef) return;
		if (!isLocal) return;
		if (!playback.session?.current) {
			if (!audioRef.paused) audioRef.pause();
			return;
		}

		const targetState = playback.session.state;
		const targetPos = playback.session.current.position ?? 0;
		const targetVol = volume;

		// Volume reconciliation.
		if (Math.abs(audioRef.volume - targetVol) > 0.01) {
			audioRef.volume = Math.max(0, Math.min(1, targetVol));
		}
		// Mute reconciliation: keep audio.muted in sync with "volume is zero".
		// HTMLMediaElement exposes a separate `muted` flag that some test code
		// (and accessibility tooling) reads directly — having it diverge from
		// the actual silence state is confusing.
		const shouldBeMuted = targetVol === 0;
		if (audioRef.muted !== shouldBeMuted) {
			audioRef.muted = shouldBeMuted;
		}

		// Position reconciliation. Allow up to 2s of drift before re-seeking
		// — normal playback advances localCurrentTime ahead of session.position
		// (server only hears about it every 500ms), so re-seeking on every
		// session update would constantly fight the audio element.
		// Skip the seek if it would land past the end of the audio: setting
		// currentTime >= duration immediately fires `ended`, which
		// onTrackEnded turns into a /api/playback/next call — cascading
		// through the album when the saved position came from a Seek call
		// targeting a longer track than this audio actually is.
		if (metadataReady && Math.abs(audioRef.currentTime - targetPos) > 2) {
			const audioDuration = audioRef.duration;
			const inRange =
				Number.isFinite(audioDuration) && audioDuration > 0 && targetPos < audioDuration;
			if (inRange) {
				audioRef.currentTime = targetPos;
			}
		}

		// Play/pause reconciliation. play() returns a Promise in modern
		// browsers and undefined in older ones / jsdom; coerce to a Promise
		// before catching so we always swallow autoplay rejections.
		if (targetState === 'playing' && audioRef.paused) {
			Promise.resolve(audioRef.play()).catch(() => {
				// Browsers reject play() outside a user gesture or on transient
				// network/decoding errors — nothing actionable here. The user
				// can press play again, which goes through the API and
				// re-triggers this effect.
			});
		}
		if (targetState !== 'playing' && !audioRef.paused) {
			audioRef.pause();
		}
	});

	// === EVENT HANDLERS — every action goes through the store, never the audio element ===
	function handleDeviceSelect(deviceId: string) {
		playback.transferPlayback(deviceId);
	}
	function handlePlayPause() {
		playback.togglePlayPause();
	}
	function handlePrevious() {
		playback.previous();
	}
	function handleNext() {
		playback.next();
	}
	function handleSeek(time: number) {
		playback.seek(time);
	}
	function handleVolumeChange(newVolume: number) {
		playback.setVolume(newVolume);
	}
	function handleToggleMute() {
		playback.toggleMute();
	}
	function openFullScreen() {
		isFullScreenOpen = true;
	}
	function closeFullScreen() {
		isFullScreenOpen = false;
	}

	// === MEDIA SESSION (lock-screen / OS media keys) ===
	// Only registered while local — when MPD is the active device, the OS
	// media controls would otherwise hijack playback into the page (iOS
	// auto-route to AirPods).
	function updateMediaSession() {
		if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return;
		if (!hasSession) {
			navigator.mediaSession.metadata = null;
			return;
		}
		const artwork = albumCover
			? [
					{ src: albumCover, sizes: '512x512', type: 'image/jpeg' },
					{ src: albumCover, sizes: '256x256', type: 'image/jpeg' }
				]
			: [];
		navigator.mediaSession.metadata = new MediaMetadata({
			title: trackTitle,
			artist: trackArtist,
			album: trackAlbum,
			artwork
		});
	}

	$effect(() => {
		void trackTitle;
		void trackArtist;
		void trackAlbum;
		void albumCover;
		void hasSession;
		void isRemote;
		if (isRemote) {
			if (typeof navigator !== 'undefined' && 'mediaSession' in navigator) {
				navigator.mediaSession.metadata = null;
				navigator.mediaSession.playbackState = 'none';
			}
			return;
		}
		updateMediaSession();
	});

	$effect(() => {
		if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return;
		if (isRemote) return;
		navigator.mediaSession.playbackState = !hasSession ? 'none' : paused ? 'paused' : 'playing';
	});

	$effect(() => {
		playback.setAudioElement(audioRef ?? null);
	});

	$effect(() => {
		if (typeof navigator === 'undefined' || !('mediaSession' in navigator)) return;
		const setActions = (handlers: {
			play: MediaSessionActionHandler | null;
			pause: MediaSessionActionHandler | null;
			previoustrack: MediaSessionActionHandler | null;
			nexttrack: MediaSessionActionHandler | null;
		}) => {
			try {
				navigator.mediaSession.setActionHandler('play', handlers.play);
				navigator.mediaSession.setActionHandler('pause', handlers.pause);
				navigator.mediaSession.setActionHandler('previoustrack', handlers.previoustrack);
				navigator.mediaSession.setActionHandler('nexttrack', handlers.nexttrack);
			} catch {
				// Some platforms throw on unsupported actions — ignore.
			}
		};
		if (isRemote) {
			setActions({ play: null, pause: null, previoustrack: null, nexttrack: null });
		} else {
			setActions({
				play: () => playback.play(),
				pause: () => playback.pause(),
				previoustrack: () => playback.previous(),
				nexttrack: () => playback.next()
			});
		}
	});

	onMount(() => {
		playback.loadSession();
		playback.loadDevices();

		const onVisibilityChange = () => {
			if (document.visibilityState === 'hidden') pushPositionNow();
		};
		window.addEventListener('beforeunload', pushPositionNow);
		document.addEventListener('visibilitychange', onVisibilityChange);

		return () => {
			window.removeEventListener('beforeunload', pushPositionNow);
			document.removeEventListener('visibilitychange', onVisibilityChange);
			playback.setAudioElement(null);
		};
	});
</script>

<!-- Hidden audio element — only mounted while playback is local. Mounting it
	even with src=undefined still lets iOS treat the page as a media app and
	auto-route to connected outputs (e.g. AirPods), so when the active device
	is remote we drop it from the DOM entirely. -->
{#if !isRemote}
	<audio
		bind:this={audioRef}
		bind:currentTime={localCurrentTime}
		bind:duration={localDuration}
		src={trackUrl}
		ontimeupdate={handleTimeUpdate}
		onloadstart={handleLoadStart}
		onloadedmetadata={handleLoadedMetadata}
		onended={() => playback.onTrackEnded()}
		data-testid="audio-element"
	></audio>
{/if}

<AudPlayFooter
	{trackTitle}
	{trackArtist}
	{trackAlbum}
	{albumCover}
	{currentTime}
	{duration}
	{paused}
	{volume}
	{muted}
	{supportsVolume}
	{deviceName}
	{isRemoteDevice}
	{showDeviceSelector}
	{devices}
	{currentDeviceId}
	onDeviceSelect={handleDeviceSelect}
	visible={hasSession}
	{isFullScreenOpen}
	onPlayPause={handlePlayPause}
	onPrevious={handlePrevious}
	onNext={handleNext}
	onSeek={handleSeek}
	onVolumeChange={handleVolumeChange}
	onToggleMute={handleToggleMute}
	onOpenFullScreen={openFullScreen}
	onCloseFullScreen={closeFullScreen}
	playLabel={m['playback.play']()}
	pauseLabel={m['playback.pause']()}
	thisDeviceLabel={m['playback.this_device']()}
	selectDeviceLabel={m['playback.select_device']()}
/>
