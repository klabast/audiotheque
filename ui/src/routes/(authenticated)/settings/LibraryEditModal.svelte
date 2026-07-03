<script lang="ts">
	import { untrack } from 'svelte';
	import { Alert, Button, Input, Label } from '$lib/components/ui';
	import type { LibraryLibraryResponse } from '$lib/api/generated/src';
	import LibraryPathsEditor from './LibraryPathsEditor.svelte';

	interface Props {
		library: Pick<LibraryLibraryResponse, 'name' | 'paths'> & { name: string; paths: string[] };
		onSave: (name: string, paths: string[]) => Promise<void>;
		onCancel: () => void;
	}

	let { library, onSave, onCancel }: Props = $props();

	let editLibraryName = $state(untrack(() => library.name));
	let editLibraryPaths: string[] = $state(untrack(() => [...library.paths]));
	let editFormError = $state<string | null>(null);
	let isUpdating = $state(false);

	function addEditPath() {
		editLibraryPaths = [...editLibraryPaths, ''];
	}

	function removeEditPath(index: number) {
		editLibraryPaths = editLibraryPaths.filter((_, i) => i !== index);
	}

	async function handleUpdate() {
		editFormError = null;

		const trimmedName = editLibraryName.trim();
		const filteredPaths = editLibraryPaths.filter((p) => p.trim() !== '');

		if (!trimmedName) {
			editFormError = 'Library name is required';
			return;
		}

		if (filteredPaths.length === 0) {
			editFormError = 'At least one path is required';
			return;
		}

		isUpdating = true;
		try {
			await onSave(trimmedName, filteredPaths);
		} catch (error) {
			console.error('Failed to update library:', error);
			editFormError = error instanceof Error ? error.message : 'Failed to update library';
		} finally {
			isUpdating = false;
		}
	}
</script>

<div
	class="modal-overlay"
	role="dialog"
	aria-modal="true"
	aria-labelledby="edit-modal-title"
	tabindex="-1"
	onkeydown={(e) => e.key === 'Escape' && onCancel()}
	data-testid="edit-library-modal"
>
	<!-- svelte-ignore a11y_click_events_have_key_events -->
	<!-- svelte-ignore a11y_no_static_element_interactions -->
	<div class="modal-backdrop" onclick={onCancel}></div>

	<div class="modal-content-lg">
		<h2 id="edit-modal-title" class="text-text-primary text-xl font-bold">Edit Library</h2>

		<div class="mt-4 space-y-4">
			<div>
				<Label for="edit-library-name">Library Name *</Label>
				<Input
					id="edit-library-name"
					type="text"
					data-testid="edit-library-name-input"
					bind:value={editLibraryName}
					placeholder="My Music Collection"
					disabled={isUpdating}
				/>
			</div>

			<div>
				<LibraryPathsEditor
					paths={editLibraryPaths}
					addButtonTestId="edit-add-path-button"
					pathInputTestIdBase="edit-library-path-input"
					removeButtonTestIdBase="edit-remove-path-button"
					disabled={isUpdating}
					onAdd={addEditPath}
					onRemove={removeEditPath}
				/>
			</div>

			{#if editFormError}
				<Alert variant="error" data-testid="edit-validation-error">
					{editFormError}
				</Alert>
			{/if}
		</div>

		<div class="mt-6 flex justify-end gap-3">
			<Button
				type="button"
				variant="ghost"
				onclick={onCancel}
				disabled={isUpdating}
				data-testid="edit-library-cancel-button"
			>
				Cancel
			</Button>
			<Button
				type="button"
				variant="primary"
				onclick={handleUpdate}
				disabled={isUpdating || !editLibraryName.trim() || editLibraryPaths.every((p) => !p.trim())}
				data-testid="edit-library-save-button"
			>
				{isUpdating ? 'Saving...' : 'Save Changes'}
			</Button>
		</div>
	</div>
</div>
