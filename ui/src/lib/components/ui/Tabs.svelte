<script lang="ts" module>
	export interface TabItem {
		label: string;
		value: string;
	}
</script>

<script lang="ts">
	import { Tabs as TabsPrimitive } from 'bits-ui';
	import type { Snippet } from 'svelte';

	interface Props {
		tabs: TabItem[];
		value?: string;
		onValueChange?: (value: string) => void;
		children: Snippet<[string]>;
	}

	let { tabs, value = $bindable(tabs[0]?.value || ''), onValueChange, children }: Props = $props();

	function handleValueChange(newValue: string) {
		value = newValue;
		onValueChange?.(newValue);
	}
</script>

<TabsPrimitive.Root {value} onValueChange={handleValueChange} class="w-full">
	<TabsPrimitive.List class="border-surface-hover flex gap-1 border-b">
		{#each tabs as tab (tab.value)}
			<TabsPrimitive.Trigger
				value={tab.value}
				class="text-text-secondary hover:text-text-primary data-[state=active]:text-text-primary data-[state=active]:border-primary relative px-4 py-3 text-sm font-medium transition-colors data-[state=active]:border-b-2"
			>
				{tab.label}
			</TabsPrimitive.Trigger>
		{/each}
	</TabsPrimitive.List>

	{#each tabs as tab (tab.value)}
		<TabsPrimitive.Content value={tab.value} class="py-6">
			{@render children(tab.value)}
		</TabsPrimitive.Content>
	{/each}
</TabsPrimitive.Root>
