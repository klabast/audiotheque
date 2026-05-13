<script lang="ts">
	import type { Snippet } from 'svelte';
	import * as m from '$lib/paraglide/messages';
	import { X } from 'lucide-svelte';

	interface Props {
		children: Snippet;
		isRightSidebarOpen: boolean;
		closeRightSidebar: () => void;
	}

	let { children, isRightSidebarOpen, closeRightSidebar }: Props = $props();
</script>

<!-- Backdrop -->
{#if isRightSidebarOpen}
	<div
		class="fixed inset-0 z-40 bg-black/50 backdrop-blur-sm"
		onclick={closeRightSidebar}
		onkeydown={(e) => e.key === 'Escape' && closeRightSidebar()}
		role="button"
		tabindex="-1"
		aria-label={m['layout.close']()}
		data-testid="right-sidebar-backdrop"
	></div>
{/if}

<!-- Slide-over panel (mobile + desktop) -->
<aside
	class="bg-surface border-surface-hover fixed inset-y-0 right-0 z-50 w-80 max-w-[85vw] border-l shadow-xl transition-transform duration-300 ease-out {isRightSidebarOpen
		? 'translate-x-0'
		: 'translate-x-full'}"
	data-testid="right-sidebar"
>
	{#if isRightSidebarOpen}
		<div class="flex h-full flex-col gap-4 p-4">
			<div class="flex items-center justify-between">
				<h2 class="text-text-primary text-lg font-bold">{m['layout.queue']()}</h2>
				<button
					onclick={closeRightSidebar}
					class="text-text-secondary hover:text-text-primary hover:bg-surface-hover flex min-h-11 min-w-11 items-center justify-center rounded transition-colors"
					aria-label={m['layout.close']()}
					data-testid="close-queue-button"
				>
					<X class="h-5 w-5" />
				</button>
			</div>
			<div class="text-text-primary flex-1 overflow-y-auto">
				{@render children()}
			</div>
		</div>
	{/if}
</aside>
