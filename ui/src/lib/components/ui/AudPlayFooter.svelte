<script lang="ts">
	import AudSeekBar from './AudSeekBar.svelte';
	import AudVolumeSlider from './AudVolumeSlider.svelte';
	import AudDeviceSelector from './AudDeviceSelector.svelte';
	import AudHomeIndicator from './AudHomeIndicator.svelte';
	import type { DeviceInfo } from '$lib/services/api';

	interface Props {
		// Track info
		trackTitle: string;
		trackArtist: string;
		trackAlbum?: string;
		albumCover: string | null;
		// Playback state
		currentTime: number;
		duration: number;
		paused: boolean;
		volume: number;
		muted: boolean;
		// Volume capability hint: when false, the active device cannot
		// physically change its volume (e.g. MPD with `mixer_type "none"`).
		// The volume slider grays out and ignores input.
		supportsVolume?: boolean;
		// Device
		deviceName: string;
		isRemoteDevice: boolean;
		showDeviceSelector?: boolean;
		devices?: DeviceInfo[];
		currentDeviceId?: string;
		onDeviceSelect?: (deviceId: string) => void;
		// Visibility
		visible: boolean;
		isFullScreenOpen?: boolean;
		// Event handlers
		onPlayPause: () => void;
		onPrevious: () => void;
		onNext: () => void;
		onSeek: (time: number) => void;
		onVolumeChange: (volume: number) => void;
		onToggleMute: () => void;
		onOpenFullScreen?: () => void;
		onCloseFullScreen?: () => void;
		// i18n labels
		playLabel?: string;
		pauseLabel?: string;
		playingOnLabel?: string;
		thisDeviceLabel?: string;
		selectDeviceLabel?: string;
	}

	let {
		trackTitle,
		trackArtist,
		trackAlbum = '',
		albumCover,
		currentTime,
		duration,
		paused,
		volume,
		muted,
		supportsVolume = true,
		deviceName,
		isRemoteDevice,
		showDeviceSelector = true,
		devices = [],
		currentDeviceId = '',
		onDeviceSelect,
		visible,
		isFullScreenOpen = false,
		onPlayPause,
		onPrevious,
		onNext,
		onSeek,
		onVolumeChange,
		onToggleMute,
		onOpenFullScreen,
		onCloseFullScreen,
		playLabel = 'Play',
		pauseLabel = 'Pause',
		playingOnLabel = 'Playing on',
		thisDeviceLabel = 'This Device',
		selectDeviceLabel = 'Select device'
	}: Props = $props();

	const progress = $derived(duration > 0 ? (currentTime / duration) * 100 : 0);

	function handleMobileBarTap(event: MouseEvent) {
		// Don't open full-screen if clicking play button
		if ((event.target as HTMLElement).closest('button')) return;
		onOpenFullScreen?.();
	}

	// Full-screen swipe-to-dismiss state
	let startY = $state(0);
	let currentY = $state(0);
	let isDragging = $state(false);

	function handleTouchStart(event: TouchEvent) {
		startY = event.touches[0].clientY;
		isDragging = true;
	}

	function handleTouchMove(event: TouchEvent) {
		if (!isDragging) return;
		currentY = event.touches[0].clientY - startY;
		if (currentY < 0) currentY = 0;
	}

	function handleTouchEnd() {
		if (currentY > 100) {
			onCloseFullScreen?.();
		}
		currentY = 0;
		isDragging = false;
	}
</script>

<footer
	class="bg-surface z-20 border-t border-border shadow-lg grid transition-[grid-template-rows] {visible
		? 'grid-rows-[1fr] duration-200 ease-out'
		: 'grid-rows-[0fr] duration-500 ease-in'}"
	data-testid="player-footer"
>
	<div class="overflow-hidden">
		<!-- Mobile: tappable card. Desktop: full-width bar -->
		<!-- svelte-ignore a11y_click_events_have_key_events a11y_no_static_element_interactions -->
		<div
			class="sm:p-3 sm:cursor-default bg-surface-alt mx-2 mb-1 rounded-xl shadow-lg cursor-pointer sm:bg-transparent sm:mx-0 sm:mb-0 sm:rounded-none sm:shadow-none"
			onclick={(e) => {
				// Only open full-screen on mobile (card area, not buttons)
				if (window.innerWidth < 640) handleMobileBarTap(e);
			}}
		>
			<!-- Mobile progress bar (thin, non-interactive) -->
			<div class="h-0.5 bg-surface-hover sm:hidden">
				<div class="h-full bg-primary transition-all" style="width: {progress}%;"></div>
			</div>

			<!-- Main content: single responsive row on mobile, 3-column grid on desktop -->
			<div
				class="p-2.5 flex items-center gap-3 sm:p-0 sm:mx-auto sm:max-w-[1400px] sm:grid sm:items-center sm:gap-6 sm:grid-cols-[300px_1fr_300px]"
			>
				<!-- Left: Album art + track info -->
				<div class="contents sm:flex sm:items-center sm:gap-4 sm:min-w-0">
					<div
						class="w-11 h-11 sm:w-16 sm:h-16 rounded-lg overflow-hidden flex-shrink-0 bg-surface-alt shadow-md"
					>
						{#if albumCover}
							<img src={albumCover} alt="Album cover" class="w-full h-full object-cover" />
						{/if}
					</div>
					<div class="min-w-0 flex-1" data-testid="player-track-info">
						<div class="text-sm font-semibold text-text-primary truncate">{trackTitle}</div>
						{#if isRemoteDevice && deviceName}
							<div
								class="text-xs text-primary truncate flex items-center gap-1 sm:hidden"
								data-testid="device-indicator"
							>
								<div class="i-lucide-speaker w-3 h-3 flex-shrink-0"></div>
								<span>{deviceName}</span>
							</div>
						{:else}
							<div class="text-xs text-text-secondary truncate">{trackArtist}</div>
						{/if}
						{#if trackAlbum}
							<div class="hidden sm:block text-xs text-text-muted truncate">{trackAlbum}</div>
						{/if}
					</div>
					<div class="hidden sm:flex items-center gap-1 flex-shrink-0">
						<button class="player-btn-muted" aria-label="Add to favorites">
							<div class="i-lucide-heart w-4 h-4"></div>
						</button>
						<button class="player-btn-muted" aria-label="More options">
							<div class="i-lucide-more-horizontal w-4 h-4"></div>
						</button>
					</div>
				</div>

				<!-- Center: Controls + seek (desktop: column layout, mobile: just play/pause) -->
				<div
					class="sm:flex sm:flex-col sm:items-center sm:gap-1 sm:max-w-[500px] sm:justify-self-center sm:w-full"
				>
					<div class="flex items-center gap-2 sm:h-10">
						<button
							class="player-btn hidden sm:flex"
							onclick={onPrevious}
							aria-label="Previous track"
							data-testid="previous-button"
						>
							<div class="i-lucide-skip-back w-5 h-5"></div>
						</button>
						<button
							class="w-11 h-11 flex items-center justify-center rounded-full text-text-primary active:bg-surface-hover sm:w-10 sm:h-10 sm:bg-action sm:text-white sm:active:opacity-80"
							onclick={onPlayPause}
							aria-label={paused ? playLabel : pauseLabel}
							data-testid="play-pause-button"
						>
							{#if paused}
								<div class="i-lucide-play w-6 h-6 ml-0.5 sm:w-5 sm:h-5"></div>
							{:else}
								<div class="i-lucide-pause w-6 h-6 sm:w-5 sm:h-5"></div>
							{/if}
						</button>
						<button
							class="player-btn hidden sm:flex"
							onclick={onNext}
							aria-label="Next track"
							data-testid="next-button"
						>
							<div class="i-lucide-skip-forward w-5 h-5"></div>
						</button>
					</div>
					<div class="hidden sm:block w-full">
						<AudSeekBar
							{currentTime}
							{duration}
							{onSeek}
							showTimes={true}
							showThumb="hover"
							testId="seek-bar"
						/>
					</div>
				</div>

				<!-- Right: Volume + Device (desktop only) -->
				<div class="hidden sm:flex flex-col gap-1">
					<div class="h-10"></div>
					<div class="flex items-center gap-3 h-8">
						<AudVolumeSlider
							{volume}
							{muted}
							{supportsVolume}
							{onVolumeChange}
							{onToggleMute}
							variant="desktop"
						/>
						{#if showDeviceSelector}
							<div class="divider"></div>
							<AudDeviceSelector
								{deviceName}
								isRemote={isRemoteDevice}
								variant="dropdown"
								{devices}
								{currentDeviceId}
								{onDeviceSelect}
								{thisDeviceLabel}
								{selectDeviceLabel}
							/>
						{/if}
					</div>
				</div>
			</div>
		</div>

		<!-- Home indicator (mobile only) -->
		<div class="sm:hidden">
			<AudHomeIndicator />
		</div>
	</div>
</footer>

<!-- Full-screen mobile player -->
{#if isFullScreenOpen}
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div
		class="fixed inset-0 z-50 bg-surface-alt flex flex-col transition-transform duration-300"
		style="transform: translateY({currentY}px)"
		ontouchstart={handleTouchStart}
		ontouchmove={handleTouchMove}
		ontouchend={handleTouchEnd}
		data-testid="player-fullscreen"
	>
		<!-- Header with collapse handle -->
		<div class="flex justify-center pt-3 pb-2">
			<button
				class="w-10 h-1 bg-text-muted/30 rounded-full"
				onclick={onCloseFullScreen}
				aria-label="Close player"
			></button>
		</div>

		<!-- Album art - large and centered -->
		<div class="flex-1 flex items-center justify-center px-8 pt-4">
			<div
				class="w-full aspect-square max-w-[340px] rounded-xl overflow-hidden shadow-2xl bg-surface"
			>
				{#if albumCover}
					<img src={albumCover} alt="Album cover" class="w-full h-full object-cover" />
				{/if}
			</div>
		</div>

		<!-- Track info -->
		<div class="px-8 pt-8 pb-4">
			<div class="flex items-start justify-between gap-4">
				<div class="min-w-0 flex-1">
					<div class="text-xl font-bold text-text-primary truncate">{trackTitle}</div>
					<div class="text-base text-text-secondary truncate">{trackArtist}</div>
					{#if trackAlbum}
						<div class="text-sm text-text-muted truncate">{trackAlbum}</div>
					{/if}
				</div>
				<button class="player-btn-mobile" aria-label="Add to favorites">
					<div class="i-lucide-heart w-6 h-6"></div>
				</button>
			</div>
		</div>

		<!-- Seek bar -->
		<div class="px-8 pb-6">
			<AudSeekBar
				{currentTime}
				{duration}
				{onSeek}
				showTimes={true}
				showThumb="always"
				testId="seek-bar-fullscreen"
			/>
		</div>

		<!-- Playback controls -->
		<div class="px-8 pb-8 flex items-center justify-center gap-8">
			<button
				class="player-btn-mobile-lg"
				onclick={onPrevious}
				aria-label="Previous track"
				data-testid="previous-button-fullscreen"
			>
				<div class="i-lucide-skip-back w-8 h-8"></div>
			</button>
			<button
				class="player-btn-mobile-play"
				onclick={onPlayPause}
				aria-label={paused ? playLabel : pauseLabel}
				data-testid="play-pause-button-fullscreen"
			>
				{#if paused}
					<div class="i-lucide-play w-9 h-9 ml-1"></div>
				{:else}
					<div class="i-lucide-pause w-9 h-9"></div>
				{/if}
			</button>
			<button
				class="player-btn-mobile-lg"
				onclick={onNext}
				aria-label="Next track"
				data-testid="next-button-fullscreen"
			>
				<div class="i-lucide-skip-forward w-8 h-8"></div>
			</button>
		</div>

		<!-- Volume -->
		<div class="px-8 pb-6">
			<AudVolumeSlider
				{volume}
				{muted}
				{supportsVolume}
				{onVolumeChange}
				{onToggleMute}
				variant="mobile"
			/>
		</div>

		<!-- Device selector -->
		{#if showDeviceSelector}
			<div class="px-6 pb-4">
				<AudDeviceSelector
					{deviceName}
					isRemote={isRemoteDevice}
					variant="card"
					{playingOnLabel}
					{devices}
					{currentDeviceId}
					{onDeviceSelect}
					{thisDeviceLabel}
					{selectDeviceLabel}
				/>
			</div>
		{/if}

		<!-- Home indicator -->
		<AudHomeIndicator />
	</div>
{/if}
