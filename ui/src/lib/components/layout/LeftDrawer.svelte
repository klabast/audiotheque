<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { APP_NAME } from '$lib/branding';
	import { Avatar } from '$lib/components/ui';
	import { routes } from '$lib/utils/navigation';
	import { Home, Settings, LogOut, X } from 'lucide-svelte';
	import * as m from '$lib/paraglide/messages';

	interface Props {
		isLeftDrawerOpen: boolean;
		closeLeftDrawer: () => void;
	}

	let { isLeftDrawerOpen, closeLeftDrawer }: Props = $props();

	function handleLogout() {
		auth.logout();
	}

	const navigationLinks = [{ label: m['common.home'](), href: routes.home, icon: Home }];
</script>

<aside
	class="bg-surface fixed inset-y-0 left-0 z-40 w-64 transform shadow-xl transition-transform duration-300 ease-out {isLeftDrawerOpen
		? 'translate-x-0'
		: '-translate-x-full'}"
>
	<div class="flex h-full flex-col">
		<div class="border-surface-hover flex h-14 items-center gap-3 border-b px-4">
			<button
				onclick={closeLeftDrawer}
				class="text-text-primary hover:bg-surface-hover flex min-h-11 min-w-11 items-center justify-center rounded transition-colors"
				aria-label={m['layout.close']()}
				data-testid="drawer-close-button"
			>
				<X class="h-5 w-5" />
			</button>
			<h2 class="text-text-primary text-lg font-bold">{APP_NAME}</h2>
		</div>

		<div class="border-surface-hover border-b px-4 py-4">
			<div class="flex items-center gap-3">
				<Avatar size="md" fallback={auth.user?.username?.[0]?.toUpperCase() || 'U'} />
				<div class="flex flex-col">
					<span class="text-text-primary text-sm font-medium">{auth.user?.username || 'User'}</span>
				</div>
			</div>
		</div>

		<nav class="flex-1 overflow-y-auto px-2 py-4">
			<div class="space-y-1">
				{#each navigationLinks as link (link.href)}
					{@const Icon = link.icon}
					<a
						href={link.href}
						onclick={closeLeftDrawer}
						class="text-text-primary hover:bg-surface-hover flex items-center gap-3 rounded-lg px-3 py-2 transition-colors"
					>
						<Icon class="h-5 w-5" />
						<span class="text-sm font-medium">{link.label}</span>
					</a>
				{/each}
			</div>
		</nav>

		<div class="border-surface-hover border-t px-2 py-4">
			<div class="text-text-secondary mb-2 px-3 text-xs font-semibold tracking-wider uppercase">
				{m['common.system']()}
			</div>
			<div class="space-y-1">
				<a
					href={routes.settings}
					onclick={closeLeftDrawer}
					class="text-text-primary hover:bg-surface-hover flex items-center gap-3 rounded-lg px-3 py-2 transition-colors"
				>
					<Settings class="h-5 w-5" />
					<span class="text-sm font-medium">{m['common.settings']()}</span>
				</a>
				<button
					onclick={handleLogout}
					class="text-error hover:bg-error/10 flex w-full items-center gap-3 rounded-lg px-3 py-2 transition-colors"
					data-testid="drawer-logout-button"
				>
					<LogOut class="h-5 w-5" />
					<span class="text-sm font-medium">{m['common.logout']()}</span>
				</button>
			</div>
		</div>
	</div>
</aside>
