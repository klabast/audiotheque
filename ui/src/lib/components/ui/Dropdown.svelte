<script lang="ts" module>
	import type { ComponentType } from 'svelte';

	export type DropdownLinkItem = {
		type: 'link';
		label: string;
		href: string;
		icon: ComponentType;
		variant?: 'default' | 'danger';
	};

	export type DropdownActionItem = {
		type: 'action';
		label: string;
		onClick: () => void;
		icon: ComponentType;
		variant?: 'default' | 'danger';
	};

	export type DropdownItem = DropdownLinkItem | DropdownActionItem;
</script>

<script lang="ts">
	import { DropdownMenu } from 'bits-ui';
	import type { Snippet } from 'svelte';

	interface Props {
		trigger: Snippet;
		items: DropdownItem[];
	}

	let { trigger, items }: Props = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger class="focus:outline-none">
		{@render trigger()}
	</DropdownMenu.Trigger>

	<DropdownMenu.Content
		class="bg-surface border-surface-hover z-50 min-w-[12rem] rounded-lg border p-1 shadow-lg"
	>
		{#each items as item (item.label)}
			{@const isDanger = item.variant === 'danger'}
			{@const itemClass = isDanger
				? 'text-error hover:bg-error/10 focus:bg-error/10'
				: 'text-text-primary hover:bg-surface-hover focus:bg-surface-hover'}
			{@const Icon = item.icon}

			{#if item.type === 'link'}
				<DropdownMenu.Item
					class="cursor-pointer rounded px-3 py-2 text-sm transition-colors focus:outline-none {itemClass}"
					data-testid="dropdown-{item.label.toLowerCase().replace(/\s+/g, '-')}"
				>
					<a href={item.href} class="flex items-center gap-2">
						<Icon class="h-4 w-4" />
						{item.label}
					</a>
				</DropdownMenu.Item>
			{:else}
				<DropdownMenu.Item
					class="flex cursor-pointer items-center gap-2 rounded px-3 py-2 text-sm transition-colors focus:outline-none {itemClass}"
					onSelect={item.onClick}
					data-testid="dropdown-{item.label.toLowerCase().replace(/\s+/g, '-')}"
				>
					<Icon class="h-4 w-4" />
					{item.label}
				</DropdownMenu.Item>
			{/if}
		{/each}
	</DropdownMenu.Content>
</DropdownMenu.Root>
