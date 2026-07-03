<script lang="ts">
	import { Button, Input } from '$lib/components/ui';

	interface Props {
		paths: string[];
		addButtonTestId: string;
		pathInputTestIdBase: string;
		removeButtonTestIdBase: string;
		disabled?: boolean;
		onAdd: () => void;
		onRemove: (index: number) => void;
	}

	let {
		paths,
		addButtonTestId,
		pathInputTestIdBase,
		removeButtonTestIdBase,
		disabled = false,
		onAdd,
		onRemove
	}: Props = $props();
</script>

<div class="flex-between mb-2">
	<span class="text-text-primary block text-sm font-medium"> Library Paths * </span>
	<Button
		type="button"
		variant="ghost"
		data-testid={addButtonTestId}
		onclick={onAdd}
		{disabled}
		class="text-sm"
	>
		+ Add Path
	</Button>
</div>

<div class="space-y-2">
	{#each paths as _path, index (index)}
		<div class="flex gap-2">
			<Input
				type="text"
				data-testid="{pathInputTestIdBase}-{index}"
				bind:value={paths[index]}
				placeholder="/path/to/music"
				class="flex-1"
				{disabled}
			/>
			{#if paths.length > 1}
				<Button
					type="button"
					variant="ghost"
					data-testid="{removeButtonTestIdBase}-{index}"
					onclick={() => onRemove(index)}
					aria-label="Remove path"
					class="px-3"
					{disabled}
				>
					🗑️
				</Button>
			{/if}
		</div>
	{/each}
</div>
