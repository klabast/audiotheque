<script lang="ts">
	import UserMenu from './UserMenu.svelte';
	import { Menu, Home, ListMusic } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';
	import { AudSearchBox } from '$lib/components/ui';

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
