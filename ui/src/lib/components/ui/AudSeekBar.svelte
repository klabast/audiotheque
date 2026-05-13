<script lang="ts">
	interface Props {
		currentTime: number;
		duration: number;
		onSeek?: (time: number) => void;
		interactive?: boolean;
		showThumb?: 'hover' | 'always' | 'never';
		showTimes?: boolean;
		testId?: string;
	}

	let {
		currentTime,
		duration,
		onSeek,
		interactive = true,
		showThumb = 'hover',
		showTimes = false,
		testId = 'seek-bar'
	}: Props = $props();

	const progress = $derived(duration > 0 ? (currentTime / duration) * 100 : 0);

	// Drag state
	let isDragging = $state(false);
	let dragPercent = $state(0);

	// Show drag preview while dragging, otherwise show actual progress
	const displayPercent = $derived(isDragging ? dragPercent : progress);
	const displayTime = $derived(isDragging ? (dragPercent / 100) * duration : currentTime);

	function formatTime(seconds: number): string {
		if (!isFinite(seconds) || isNaN(seconds)) return '0:00';
		const mins = Math.floor(seconds / 60);
		const secs = Math.floor(seconds % 60);
		return `${mins}:${secs.toString().padStart(2, '0')}`;
	}

	function getPercent(event: PointerEvent, target: HTMLElement): number {
		const rect = target.getBoundingClientRect();
		return Math.max(0, Math.min(100, ((event.clientX - rect.left) / rect.width) * 100));
	}

	function handlePointerDown(event: PointerEvent) {
		if (!interactive || !onSeek) return;
		const target = event.currentTarget as HTMLElement;
		target.setPointerCapture(event.pointerId);
		isDragging = true;
		dragPercent = getPercent(event, target);
	}

	function handlePointerMove(event: PointerEvent) {
		if (!isDragging) return;
		const target = event.currentTarget as HTMLElement;
		dragPercent = getPercent(event, target);
	}

	function handlePointerUp() {
		if (!isDragging) return;
		const seekTime = (dragPercent / 100) * duration;
		isDragging = false;
		onSeek?.(seekTime);
	}

	function handleClick(event: MouseEvent) {
		if (!interactive || !onSeek) return;
		const target = event.currentTarget as HTMLDivElement;
		const rect = target.getBoundingClientRect();
		const percent = Math.max(0, Math.min(1, (event.clientX - rect.left) / rect.width));
		onSeek(percent * duration);
	}

	function handleKeydown(event: KeyboardEvent) {
		if (!interactive || !onSeek) return;
		if (event.key === 'ArrowRight') {
			onSeek(Math.min(duration, currentTime + 5));
		} else if (event.key === 'ArrowLeft') {
			onSeek(Math.max(0, currentTime - 5));
		}
	}
</script>

{#if showTimes}
	<div class="flex items-center gap-2 w-full">
		<span class="min-w-[40px] text-xs tabular-nums text-text-muted text-right">
			{formatTime(displayTime)}
		</span>
		<div
			class="player-seek group relative h-1 flex-1 rounded-full bg-surface-hover touch-none {interactive
				? 'cursor-pointer'
				: ''}"
			onclick={interactive ? handleClick : undefined}
			onpointerdown={interactive ? handlePointerDown : undefined}
			onpointermove={interactive ? handlePointerMove : undefined}
			onpointerup={interactive ? handlePointerUp : undefined}
			onkeydown={interactive ? handleKeydown : undefined}
			role={interactive ? 'slider' : 'progressbar'}
			aria-label="Seek"
			aria-valuenow={currentTime}
			aria-valuemin={0}
			aria-valuemax={duration}
			tabindex={interactive ? 0 : -1}
			data-testid={testId}
		>
			<div
				class="player-seek-progress absolute top-0 left-0 h-full rounded-full bg-primary transition-all"
				style="width: {displayPercent}%"
			></div>
			{#if showThumb !== 'never'}
				<div
					class="player-seek-thumb absolute top-1/2 h-3 w-3 -translate-y-1/2 rounded-full bg-text-primary transition-opacity {isDragging
						? 'opacity-100 shadow-md'
						: showThumb === 'hover'
							? 'opacity-0 group-hover:opacity-100'
							: 'opacity-100 shadow-md'}"
					style="left: calc({displayPercent}% - 6px)"
				></div>
			{/if}
		</div>
		<span class="min-w-[40px] text-xs tabular-nums text-text-muted">
			{formatTime(duration)}
		</span>
	</div>
{:else}
	<div
		class="player-seek group relative h-1 flex-1 rounded-full bg-surface-hover touch-none {interactive
			? 'cursor-pointer'
			: ''}"
		onclick={interactive ? handleClick : undefined}
		onpointerdown={interactive ? handlePointerDown : undefined}
		onpointermove={interactive ? handlePointerMove : undefined}
		onpointerup={interactive ? handlePointerUp : undefined}
		onkeydown={interactive ? handleKeydown : undefined}
		role={interactive ? 'slider' : 'progressbar'}
		aria-label="Seek"
		aria-valuenow={currentTime}
		aria-valuemin={0}
		aria-valuemax={duration}
		tabindex={interactive ? 0 : -1}
		data-testid={testId}
	>
		<div
			class="player-seek-progress absolute top-0 left-0 h-full rounded-full bg-primary transition-all"
			style="width: {displayPercent}%"
		></div>
		{#if showThumb !== 'never'}
			<div
				class="player-seek-thumb absolute top-1/2 h-3 w-3 -translate-y-1/2 rounded-full bg-text-primary transition-opacity {isDragging
					? 'opacity-100 shadow-md'
					: showThumb === 'hover'
						? 'opacity-0 group-hover:opacity-100'
						: 'opacity-100 shadow-md'}"
				style="left: calc({displayPercent}% - 6px)"
			></div>
		{/if}
	</div>
{/if}
