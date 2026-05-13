<script lang="ts" module>
	import type { ComponentType } from 'svelte';

	export type DropdownButtonItem = {
		label: string;
		description?: string;
		icon?: ComponentType;
		onClick: () => void;
		testid?: string;
	};
</script>

<script lang="ts">
	import { DropdownMenu } from 'bits-ui';
	import { ChevronDown } from 'lucide-svelte';

	interface Props {
		label: string;
		icon?: ComponentType;
		items: DropdownButtonItem[];
		variant?: 'primary' | 'secondary' | 'ghost';
		onMainClick?: () => void;
		testid?: string;
	}

	let { label, icon, items, variant = 'secondary', onMainClick, testid }: Props = $props();

	// Base button styles (matching btn shortcut but without rounded-lg for split button)
	const baseStyles =
		'px-4 py-3 font-medium transition-colors duration-200 focus:ring-2 focus:ring-offset-2 focus:outline-none disabled:(cursor-not-allowed opacity-50)';

	const variantStyles = {
		primary:
			'bg-primary hover:bg-primary-muted text-text-primary focus:ring-primary focus:ring-offset-background',
		secondary:
			'bg-surface hover:bg-surface-hover text-text-primary focus:ring-surface focus:ring-offset-background',
		ghost: 'bg-transparent hover:bg-surface-hover text-text-secondary focus:ring-surface'
	};
</script>

<DropdownMenu.Root>
	<div class="flex">
		<!-- Main Button -->
		{#if onMainClick}
			<button
				type="button"
				onclick={onMainClick}
				data-testid={testid}
				class="rounded-l-lg {baseStyles} {variantStyles[variant]}"
			>
				<span class="flex items-center gap-2">
					{#if icon}
						{@const Icon = icon}
						<Icon class="h-4 w-4" />
					{/if}
					{label}
				</span>
			</button>
		{:else}
			<div class="rounded-l-lg px-4 py-3 font-medium {variantStyles[variant]}" data-testid={testid}>
				<span class="flex items-center gap-2">
					{#if icon}
						{@const Icon = icon}
						<Icon class="h-4 w-4" />
					{/if}
					{label}
				</span>
			</div>
		{/if}

		<!-- Dropdown Trigger -->
		<DropdownMenu.Trigger
			class="rounded-r-lg border-l border-black/10 px-2 py-3 transition-colors duration-200 focus:ring-2 focus:ring-offset-2 focus:outline-none {variantStyles[
				variant
			]}"
		>
			<ChevronDown class="h-4 w-4" />
		</DropdownMenu.Trigger>
	</div>

	<DropdownMenu.Content
		class="bg-surface border-surface-hover z-50 min-w-[16rem] rounded-lg border p-1 shadow-lg"
	>
		{#each items as item (item.label)}
			<DropdownMenu.Item
				class="text-text-primary hover:bg-surface-hover focus:bg-surface-hover cursor-pointer rounded px-3 py-2 transition-colors focus:outline-none"
				onSelect={item.onClick}
				data-testid={item.testid}
			>
				<div class="flex gap-3">
					{#if item.icon}
						{@const ItemIcon = item.icon}
						<ItemIcon class="text-text-secondary h-5 w-5 flex-shrink-0" />
					{/if}
					<div class="flex flex-col">
						<span class="text-sm font-medium">{item.label}</span>
						{#if item.description}
							<span class="text-text-secondary mt-0.5 text-xs">{item.description}</span>
						{/if}
					</div>
				</div>
			</DropdownMenu.Item>
		{/each}
	</DropdownMenu.Content>
</DropdownMenu.Root>
