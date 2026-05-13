<script lang="ts">
	interface Props {
		volume: number;
		muted: boolean;
		// supportsVolume=false greys the slider/mute and ignores input. Used
		// for MPD devices with `mixer_type "none"` (e.g. HiFiBerry default)
		// where setvol always fails server-side.
		supportsVolume?: boolean;
		onVolumeChange: (volume: number) => void;
		onToggleMute: () => void;
		variant?: 'desktop' | 'mobile';
		muteLabel?: string;
		unmuteLabel?: string;
	}

	let {
		volume,
		muted,
		supportsVolume = true,
		onVolumeChange,
		onToggleMute,
		variant = 'desktop',
		muteLabel = 'Mute',
		unmuteLabel = 'Unmute'
	}: Props = $props();

	const displayVolume = $derived(muted ? 0 : volume);

	let isDragging = $state(false);

	function getPercent(event: PointerEvent, target: HTMLElement): number {
		const rect = target.getBoundingClientRect();
		return Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
	}

	function handlePointerDown(event: PointerEvent) {
		if (!supportsVolume) return;
		const target = event.currentTarget as HTMLElement;
		target.setPointerCapture(event.pointerId);
		isDragging = true;
		onVolumeChange(getPercent(event, target));
	}

	function handlePointerMove(event: PointerEvent) {
		if (!isDragging || !supportsVolume) return;
		const target = event.currentTarget as HTMLElement;
		onVolumeChange(getPercent(event, target));
	}

	function handlePointerUp() {
		isDragging = false;
	}

	function handleKeydown(event: KeyboardEvent) {
		if (!supportsVolume) return;
		if (event.key === 'ArrowRight' || event.key === 'ArrowUp') {
			onVolumeChange(Math.min(1, volume + 0.1));
		} else if (event.key === 'ArrowLeft' || event.key === 'ArrowDown') {
			onVolumeChange(Math.max(0, volume - 0.1));
		}
	}

	function handleMuteClick() {
		if (!supportsVolume) return;
		onToggleMute();
	}
</script>

{#if variant === 'desktop'}
	<!-- Desktop: mute button + horizontal slider -->
	<div
		class="flex items-center gap-2 {!supportsVolume ? 'opacity-40' : ''}"
		data-testid="volume-control"
		data-supports-volume={supportsVolume}
	>
		<button
			class="player-btn flex-shrink-0"
			onclick={handleMuteClick}
			disabled={!supportsVolume}
			aria-label={muted ? unmuteLabel : muteLabel}
			aria-disabled={!supportsVolume}
			data-testid="mute-button"
		>
			{#if muted || volume === 0}
				<div class="i-lucide-volume-x w-5 h-5"></div>
			{:else if volume < 0.5}
				<div class="i-lucide-volume-1 w-5 h-5"></div>
			{:else}
				<div class="i-lucide-volume-2 w-5 h-5"></div>
			{/if}
		</button>
		<div
			class="volume-slider group w-20 h-8 {supportsVolume
				? 'cursor-pointer'
				: 'cursor-not-allowed'} relative flex items-center flex-shrink-0 touch-none"
			onpointerdown={handlePointerDown}
			onpointermove={handlePointerMove}
			onpointerup={handlePointerUp}
			onkeydown={handleKeydown}
			role="slider"
			aria-label="Volume"
			aria-disabled={!supportsVolume}
			aria-valuenow={displayVolume * 100}
			aria-valuemin={0}
			aria-valuemax={100}
			tabindex={supportsVolume ? 0 : -1}
			data-testid="volume-slider"
		>
			<div class="w-full h-1 bg-surface-hover rounded-full relative">
				<div
					class="h-full bg-text-secondary rounded-full"
					style="width: {displayVolume * 100}%;"
				></div>
				<div
					class="absolute top-1/2 -translate-y-1/2 w-3 h-3 bg-text-primary rounded-full transition-opacity {isDragging
						? 'opacity-100'
						: 'opacity-0 group-hover:opacity-100'}"
					style="left: calc({displayVolume * 100}% - 6px);"
				></div>
			</div>
		</div>
	</div>
{:else}
	<!-- Mobile: full-width slider with mute toggle and high-volume icon -->
	<div
		class="flex items-center gap-3 w-full {!supportsVolume ? 'opacity-40' : ''}"
		data-testid="volume-control"
		data-supports-volume={supportsVolume}
	>
		<button
			class="flex-shrink-0 w-6 h-6 flex items-center justify-center text-text-muted active:text-text-primary"
			onclick={handleMuteClick}
			disabled={!supportsVolume}
			aria-label={muted ? unmuteLabel : muteLabel}
			aria-disabled={!supportsVolume}
			data-testid="mute-button"
		>
			{#if muted || volume === 0}
				<div class="i-lucide-volume-x w-4 h-4"></div>
			{:else}
				<div class="i-lucide-volume-1 w-4 h-4"></div>
			{/if}
		</button>
		<div
			class="flex-1 h-1 bg-surface-hover rounded-full relative {supportsVolume
				? 'cursor-pointer'
				: 'cursor-not-allowed'} touch-none"
			onpointerdown={handlePointerDown}
			onpointermove={handlePointerMove}
			onpointerup={handlePointerUp}
			onkeydown={handleKeydown}
			role="slider"
			aria-label="Volume"
			aria-disabled={!supportsVolume}
			aria-valuenow={displayVolume * 100}
			aria-valuemin={0}
			aria-valuemax={100}
			tabindex={supportsVolume ? 0 : -1}
			data-testid="volume-slider"
		>
			<div
				class="h-full bg-text-secondary rounded-full"
				style="width: {displayVolume * 100}%;"
			></div>
			<div
				class="absolute top-1/2 -translate-y-1/2 w-4 h-4 bg-text-primary rounded-full shadow-md"
				style="left: calc({displayVolume * 100}% - 8px);"
			></div>
		</div>
		<div class="i-lucide-volume-2 w-4 h-4 text-text-muted flex-shrink-0"></div>
	</div>
{/if}
