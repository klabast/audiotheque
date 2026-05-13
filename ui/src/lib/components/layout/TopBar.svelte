<script lang="ts">
	import UserMenu from './UserMenu.svelte';
	import { Menu, Home, ListMusic, Loader2 } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';
	import { AudSearchBox } from '$lib/components/ui';
	import { scan } from '$lib/stores/scan.svelte';

	interface Props {
		toggleLeftDrawer: () => void;
		toggleRightSidebar: () => void;
	}

	let { toggleLeftDrawer, toggleRightSidebar }: Props = $props();
</script>

<header class="bg-surface border-surface-hover flex h-14 items-center gap-4 border-b px-4">
	<div class="flex items-center">
		<button
			onclick={toggleLeftDrawer}
			class="text-text-primary hover:bg-surface-hover flex min-h-11 min-w-11 items-center justify-center rounded transition-colors"
			aria-label={m['layout.menu']()}
			data-testid="hamburger-menu-button"
		>
			<Menu class="h-5 w-5" />
		</button>
		<a
			href="/"
			class="text-text-primary hover:bg-surface-hover ml-2 flex min-h-11 min-w-11 items-center justify-center rounded transition-colors md:ml-4"
		>
			<Home class="h-5 w-5" />
		</a>
	</div>

	<div class="flex flex-1 justify-center">
		<AudSearchBox />
	</div>

	<div class="flex items-center gap-2">
		{#if scan.isAnyScanRunning}
			<span
				class="bg-surface-hover text-text-secondary hidden items-center gap-2 rounded-full px-3 py-1 text-xs sm:inline-flex"
				role="status"
				aria-label={m['library.scan.in_progress_aria']()}
				data-testid="topbar-scanning-indicator"
			>
				<Loader2 class="h-3.5 w-3.5 animate-spin" aria-hidden="true" />
				{m['library.scan.in_progress']()}
			</span>
		{/if}
		<button
			onclick={toggleRightSidebar}
			class="text-text-primary hover:bg-surface-hover flex min-h-11 min-w-11 items-center justify-center rounded transition-colors"
			aria-label={m['layout.queue.toggle']()}
			data-testid="toggle-queue-button"
		>
			<ListMusic class="h-5 w-5" />
		</button>
		<div class="hidden md:block">
			<UserMenu />
		</div>
	</div>
</header>
