<script lang="ts">
	import { auth } from '$lib/stores/auth.svelte';
	import { Avatar, Dropdown, type DropdownItem } from '$lib/components/ui';
	import { routes } from '$lib/utils/navigation';
	import { LogOut, Settings } from 'lucide-svelte';

	function handleLogout() {
		auth.logout();
	}

	const menuItems: DropdownItem[] = [
		{
			type: 'link',
			label: 'Settings',
			href: routes.settings,
			icon: Settings
		},
		{
			type: 'action',
			label: 'Logout',
			onClick: handleLogout,
			icon: LogOut,
			variant: 'danger'
		}
	];
</script>

<Dropdown items={menuItems}>
	{#snippet trigger()}
		<button
			data-testid="user-menu-trigger"
			class="bg-surface-hover hover:bg-surface flex items-center gap-2 rounded-full px-3 py-1.5 transition-colors"
		>
			<Avatar size="sm" fallback={auth.user?.username?.[0]?.toUpperCase() || 'U'} />
			<span class="text-sm font-medium">
				{auth.user?.username || 'User'}
			</span>
		</button>
	{/snippet}
</Dropdown>
