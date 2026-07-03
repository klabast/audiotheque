<script lang="ts">
	import { Alert, Button, Input, Label } from '$lib/components/ui';
	import LibraryPathsEditor from './LibraryPathsEditor.svelte';

	interface Props {
		onSubmit: (name: string, paths: string[]) => Promise<void>;
		onCancel: () => void;
	}

	let { onSubmit, onCancel }: Props = $props();

	let libraryName = $state('');
	let libraryPaths: string[] = $state(['']);
	let formError = $state<string | null>(null);

	function addPath() {
		libraryPaths = [...libraryPaths, ''];
	}

	function removePath(index: number) {
		libraryPaths = libraryPaths.filter((_, i) => i !== index);
	}

	async function handleSubmit() {
		formError = null;

		const trimmedName = libraryName.trim();
		const filteredPaths = libraryPaths.filter((p) => p.trim() !== '');

		if (!trimmedName) {
			formError = 'Library name is required';
			return;
		}

		if (filteredPaths.length === 0) {
			formError = 'At least one path is required';
			return;
		}

		try {
			await onSubmit(trimmedName, filteredPaths);
		} catch (error) {
			console.error('Failed to create library:', error);
			formError = error instanceof Error ? error.message : 'Failed to create library';
		}
	}
</script>

<div class="card-bordered">
	<h3 class="text-text-primary mb-4 text-lg font-semibold">Create New Library</h3>

	<div class="space-y-4">
		<div>
			<Label for="library-name">Library Name *</Label>
			<Input
				id="library-name"
				type="text"
				data-testid="library-name-input"
				bind:value={libraryName}
				placeholder="My Music Collection"
			/>
		</div>

		<div>
			<LibraryPathsEditor
				paths={libraryPaths}
				addButtonTestId="add-path-button"
				pathInputTestIdBase="library-path-input"
				removeButtonTestIdBase="remove-path-button"
				onAdd={addPath}
				onRemove={removePath}
			/>

			<p class="text-text-secondary mt-2 text-xs">ℹ️ Multiple paths will be scanned together</p>
		</div>

		{#if formError}
			<Alert variant="error" data-testid="validation-error">
				{formError}
			</Alert>
		{/if}

		<div class="flex justify-end gap-2">
			<Button type="button" variant="ghost" data-testid="cancel-library-button" onclick={onCancel}>
				Cancel
			</Button>
			<Button
				type="button"
				variant="primary"
				data-testid="save-library-button"
				onclick={handleSubmit}
			>
				Create Library
			</Button>
		</div>
	</div>
</div>
