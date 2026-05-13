<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { auth } from '$lib/stores/auth.svelte';
	import { api } from '$lib/services/api';
	import { scan } from '$lib/stores/scan.svelte';
	import {
		Alert,
		Button,
		ConfirmDeleteModal,
		DropdownButton,
		Input,
		Label
	} from '$lib/components/ui';
	import { Pencil, RefreshCw, RotateCcw, Trash2 } from 'lucide-svelte';
	import type { LibraryLibraryResponse } from '$lib/api/generated/src';
	import * as m from '$lib/paraglide/messages';

	interface LibraryWithStats extends LibraryLibraryResponse {
		id: number; // Override to make id required
		name: string; // Override to make name required
		paths: string[]; // Override to make paths required
		trackCount?: number;
		albumCount?: number;
		lastScanTime?: string;
	}

	let libraries: LibraryWithStats[] | undefined = $state(undefined);
	let loading = $state(true);
	let showForm = $state(false);

	// Form state
	let libraryName = $state('');
	let libraryPaths: string[] = $state(['']);

	// Scan progress lives in the global scan store (so it survives nav). The
	// store exposes a reactive SvelteMap; we just read from it.
	const scanProgress = scan.activeScans;

	// Scan error state - map of libraryId -> error message
	let scanErrors = new SvelteMap<number, string>();

	// Refresh the library list when the backend signals that a library's
	// contents changed (track added, scan completed).
	const offLibraryUpdated = scan.onLibraryUpdated(async () => {
		try {
			libraries = (await api.listLibraries()) as LibraryWithStats[];
		} catch (err) {
			console.error('Failed to refresh libraries after update:', err);
		}
	});
	onDestroy(() => offLibraryUpdated());

	// Delete modal state
	let deleteModalOpen = $state(false);
	let libraryToDelete: LibraryWithStats | null = $state(null);
	let isDeleting = $state(false);

	// Edit modal state
	let editModalOpen = $state(false);
	let libraryToEdit: LibraryWithStats | null = $state(null);
	let editLibraryName = $state('');
	let editLibraryPaths: string[] = $state(['']);
	let isUpdating = $state(false);

	// Form error state
	let formError = $state<string | null>(null);
	let editFormError = $state<string | null>(null);

	onMount(async () => {
		try {
			libraries = (await api.listLibraries()) as LibraryWithStats[];
		} catch (error) {
			console.error('Failed to load libraries:', error);
			libraries = [];
		} finally {
			loading = false;
		}
	});

	function toggleForm() {
		showForm = !showForm;
		if (showForm) {
			// Reset form
			libraryName = '';
			libraryPaths = [''];
			formError = null;
		}
	}

	function addPath() {
		libraryPaths = [...libraryPaths, ''];
	}

	function removePath(index: number) {
		libraryPaths = libraryPaths.filter((_, i) => i !== index);
	}

	async function handleSubmit() {
		// Clear previous error
		formError = null;

		// Client-side validation
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
			await api.createLibrary(trimmedName, filteredPaths);

			// The new library auto-scans on creation; progress flows through
			// the global scan store via the app-wide WebSocket subscription,
			// and library-updated events trigger refreshes via onLibraryUpdated.

			// Close form before the list refresh so the user isn't stuck on it
			showForm = false;
			libraries = (await api.listLibraries()) as LibraryWithStats[];
		} catch (error) {
			console.error('Failed to create library:', error);
			// Extract error message from API response
			if (error instanceof Error) {
				formError = error.message;
			} else {
				formError = 'Failed to create library';
			}
		}
	}

	async function handleScan(libraryId: number) {
		try {
			// Clear any previous errors
			scanErrors.delete(libraryId);

			// Progress and completion are picked up by the global scan store,
			// not by a per-call subscription — no local subscribe needed.
			await api.startScan(libraryId);
		} catch (error) {
			console.error('Failed to start scan:', error);
			// Store error message
			const errorMessage = error instanceof Error ? error.message : 'Unknown error';
			scanErrors.set(libraryId, errorMessage);
		}
	}

	function openDeleteModal(library: LibraryWithStats) {
		libraryToDelete = library;
		deleteModalOpen = true;
	}

	function closeDeleteModal() {
		deleteModalOpen = false;
		libraryToDelete = null;
	}

	async function handleDelete() {
		if (!libraryToDelete) return;

		isDeleting = true;
		try {
			await api.deleteLibrary(libraryToDelete.id);
			closeDeleteModal();
			libraries = (await api.listLibraries()) as LibraryWithStats[];
		} catch (error) {
			console.error('Failed to delete library:', error);
		} finally {
			isDeleting = false;
		}
	}

	function openEditModal(library: LibraryWithStats) {
		libraryToEdit = library;
		editLibraryName = library.name;
		editLibraryPaths = [...library.paths];
		editFormError = null;
		editModalOpen = true;
	}

	function closeEditModal() {
		editModalOpen = false;
		libraryToEdit = null;
		editLibraryName = '';
		editLibraryPaths = [''];
	}

	function addEditPath() {
		editLibraryPaths = [...editLibraryPaths, ''];
	}

	function removeEditPath(index: number) {
		editLibraryPaths = editLibraryPaths.filter((_, i) => i !== index);
	}

	async function handleUpdate() {
		if (!libraryToEdit) return;

		// Clear previous error
		editFormError = null;

		// Client-side validation
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
			await api.updateLibrary(libraryToEdit.id, trimmedName, filteredPaths);
			// Close the modal as soon as the update has succeeded; refreshing the
			// list is independent and shouldn't keep the user staring at the form.
			closeEditModal();
			libraries = (await api.listLibraries()) as LibraryWithStats[];
		} catch (error) {
			console.error('Failed to update library:', error);
			// Extract error message from API response
			if (error instanceof Error) {
				editFormError = error.message;
			} else {
				editFormError = 'Failed to update library';
			}
		} finally {
			isUpdating = false;
		}
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-text-primary text-2xl font-bold">Library Settings</h2>
		<p class="text-text-secondary mt-1 text-sm">Manage your music libraries</p>
	</div>

	{#if showForm}
		<!-- Create Library Form -->
		<div class="card-bordered">
			<h3 class="text-text-primary mb-4 text-lg font-semibold">Create New Library</h3>

			<div class="space-y-4">
				<!-- Library Name -->
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

				<!-- Library Paths -->
				<div>
					<div class="flex-between mb-2">
						<span class="text-text-primary block text-sm font-medium"> Library Paths * </span>
						<Button
							type="button"
							variant="ghost"
							data-testid="add-path-button"
							onclick={addPath}
							class="text-sm"
						>
							+ Add Path
						</Button>
					</div>

					<div class="space-y-2">
						{#each libraryPaths as _path, index (index)}
							<div class="flex gap-2">
								<Input
									type="text"
									data-testid="library-path-input-{index}"
									bind:value={libraryPaths[index]}
									placeholder="/path/to/music"
									class="flex-1"
								/>
								{#if libraryPaths.length > 1}
									<Button
										type="button"
										variant="ghost"
										data-testid="remove-path-button-{index}"
										onclick={() => removePath(index)}
										aria-label="Remove path"
										class="px-3"
									>
										🗑️
									</Button>
								{/if}
							</div>
						{/each}
					</div>

					<p class="text-text-secondary mt-2 text-xs">ℹ️ Multiple paths will be scanned together</p>
				</div>

				<!-- Form Error -->
				{#if formError}
					<Alert variant="error" data-testid="validation-error">
						{formError}
					</Alert>
				{/if}

				<!-- Actions -->
				<div class="flex justify-end gap-2">
					<Button
						type="button"
						variant="ghost"
						data-testid="cancel-library-button"
						onclick={toggleForm}
					>
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
	{/if}

	<!-- Library List -->
	{#if loading}
		<div class="card-bordered" data-testid="loading-libraries">
			<p class="text-text-secondary">Loading libraries...</p>
		</div>
	{:else if !libraries || libraries.length === 0}
		<div class="card-bordered">
			<p class="text-text-secondary">No libraries configured yet.</p>
		</div>
	{:else}
		<div class="space-y-4">
			{#each libraries as library (library.id)}
				<div class="card-bordered" data-testid="library-item-{library.name}">
					<div class="flex items-start justify-between">
						<div class="flex-1">
							<h3 class="text-text-primary text-lg font-semibold">{library.name}</h3>

							<!-- Paths -->
							<div
								class="text-text-secondary mt-2 space-y-1 text-sm"
								data-testid="library-paths-{library.id}"
							>
								{#each library.paths as path, index (path)}
									<div
										class="flex items-center gap-2"
										data-testid="library-path-{library.id}-{index}"
									>
										<span>📁</span>
										<span data-testid="library-path-value-{library.id}-{index}">{path}</span>
									</div>
								{/each}
							</div>

							<!-- Statistics -->
							<div class="text-text-secondary mt-3 flex gap-4 text-sm">
								<span data-testid="library-track-count-{library.id}"
									>{library.trackCount} tracks</span
								>
								<span data-testid="library-album-count-{library.id}"
									>{library.albumCount} albums</span
								>
								{#if library.lastScanTime}
									<span>Last scanned: {new Date(library.lastScanTime).toLocaleString()}</span>
								{/if}
							</div>

							<!-- Scan Error -->
							{#if scanErrors.has(library.id)}
								<div class="mt-4">
									<Alert variant="error" data-testid="scan-error-message-{library.id}">
										Error: {scanErrors.get(library.id)}
									</Alert>
								</div>
							{/if}

							<!-- Scan Progress -->
							{#if scanProgress.has(library.id) && scanProgress.get(library.id)?.status === 'running'}
								{@const progress = scanProgress.get(library.id)}
								{#if progress}
									{@const percentage =
										progress.totalFiles > 0
											? Math.round((progress.processedFiles / progress.totalFiles) * 100)
											: 0}
									<div class="mt-4" data-testid="scan-progress-bar-{library.id}">
										<div class="flex-between mb-2 text-sm">
											<span
												class="text-text-secondary"
												data-testid="scan-progress-file-count-{library.id}"
											>
												{progress.processedFiles} / {progress.totalFiles} files
											</span>
											<span
												class="text-text-primary font-medium"
												data-testid="scan-progress-percentage-{library.id}"
											>
												{percentage}%
											</span>
										</div>
										<div class="progress-track">
											<div class="progress-fill duration-300" style="width: {percentage}%"></div>
										</div>
										{#if progress.currentFile}
											<div
												class="text-text-secondary mt-2 truncate text-xs"
												data-testid="scan-progress-current-file-{library.id}"
											>
												{progress.currentFile.split('/').pop()}
											</div>
										{/if}
										<div
											class="text-text-secondary mt-2 flex gap-3 text-xs"
											data-testid="scan-progress-stats-{library.id}"
										>
											<span>Added: {progress.tracksAdded}</span>
											<span>Updated: {progress.tracksUpdated}</span>
											<span>Errors: {progress.errors}</span>
										</div>
									</div>
								{/if}
							{/if}

							<!-- Scan Complete Message -->
							{#if scanProgress.has(library.id) && scanProgress.get(library.id)?.status === 'completed'}
								{@const progress = scanProgress.get(library.id)}
								{#if progress}
									<div class="mt-4">
										<Alert variant="success" data-testid="library-scan-complete">
											✓ Scan completed: {progress.tracksAdded} tracks added, {progress.tracksUpdated}
											updated
										</Alert>
									</div>
								{/if}
							{/if}
						</div>

						<!-- Action Buttons (admin only) -->
						{#if auth.user?.isAdmin}
							<div class="flex gap-2">
								<Button
									type="button"
									variant="ghost"
									data-testid="edit-library-button-{library.id}"
									onclick={() => openEditModal(library)}
									class="px-3"
								>
									<Pencil class="h-4 w-4" />
								</Button>

								<DropdownButton
									label="Scan"
									icon={RefreshCw}
									variant="ghost"
									onMainClick={() => handleScan(library.id)}
									testid="scan-library-button-{library.id}"
									items={[
										{
											label: 'Rebuild',
											description: 'Delete all data and rescan from scratch',
											icon: RotateCcw,
											onClick: () => {},
											testid: 'rebuild-library-button-{library.id}'
										}
									]}
								/>

								<Button
									type="button"
									variant="ghost"
									data-testid="delete-library-button-{library.id}"
									onclick={() => openDeleteModal(library)}
									class="px-3"
								>
									<Trash2 class="h-4 w-4" />
								</Button>
							</div>
						{/if}
					</div>
				</div>
			{/each}
		</div>
	{/if}

	<!-- Create Button (visible for all users, disabled for non-admins) -->
	{#if !showForm}
		<div class="flex justify-end">
			<Button
				type="button"
				variant="primary"
				data-testid="create-library-button"
				onclick={toggleForm}
				disabled={!auth.user?.isAdmin}
				title={!auth.user?.isAdmin ? m['library.settings.admin_only_tooltip']() : ''}
			>
				Create Library
			</Button>
		</div>
	{/if}
</div>

<!-- Delete Confirmation Modal -->
{#if libraryToDelete}
	<ConfirmDeleteModal
		isOpen={deleteModalOpen}
		title="Delete Library"
		itemName={libraryToDelete.name}
		description="This will permanently delete the library and all its indexed tracks. This action cannot be undone."
		onConfirm={handleDelete}
		onCancel={closeDeleteModal}
		{isDeleting}
		testidPrefix="delete-library"
	/>
{/if}

<!-- Edit Library Modal -->
{#if editModalOpen && libraryToEdit}
	<div
		class="modal-overlay"
		role="dialog"
		aria-modal="true"
		aria-labelledby="edit-modal-title"
		tabindex="-1"
		onkeydown={(e) => e.key === 'Escape' && closeEditModal()}
		data-testid="edit-library-modal"
	>
		<!-- svelte-ignore a11y_click_events_have_key_events -->
		<!-- svelte-ignore a11y_no_static_element_interactions -->
		<div class="modal-backdrop" onclick={closeEditModal}></div>

		<div class="modal-content-lg">
			<h2 id="edit-modal-title" class="text-text-primary text-xl font-bold">Edit Library</h2>

			<div class="mt-4 space-y-4">
				<!-- Library Name -->
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

				<!-- Library Paths -->
				<div>
					<div class="flex-between mb-2">
						<span class="text-text-primary block text-sm font-medium"> Library Paths * </span>
						<Button
							type="button"
							variant="ghost"
							data-testid="edit-add-path-button"
							onclick={addEditPath}
							disabled={isUpdating}
							class="text-sm"
						>
							+ Add Path
						</Button>
					</div>

					<div class="space-y-2">
						{#each editLibraryPaths as _path, index (index)}
							<div class="flex gap-2">
								<Input
									type="text"
									data-testid="edit-library-path-input-{index}"
									bind:value={editLibraryPaths[index]}
									placeholder="/path/to/music"
									class="flex-1"
									disabled={isUpdating}
								/>
								{#if editLibraryPaths.length > 1}
									<Button
										type="button"
										variant="ghost"
										data-testid="edit-remove-path-button-{index}"
										onclick={() => removeEditPath(index)}
										aria-label="Remove path"
										class="px-3"
										disabled={isUpdating}
									>
										🗑️
									</Button>
								{/if}
							</div>
						{/each}
					</div>
				</div>

				<!-- Edit Form Error -->
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
					onclick={closeEditModal}
					disabled={isUpdating}
					data-testid="edit-library-cancel-button"
				>
					Cancel
				</Button>
				<Button
					type="button"
					variant="primary"
					onclick={handleUpdate}
					disabled={isUpdating ||
						!editLibraryName.trim() ||
						editLibraryPaths.every((p) => !p.trim())}
					data-testid="edit-library-save-button"
				>
					{isUpdating ? 'Saving...' : 'Save Changes'}
				</Button>
			</div>
		</div>
	</div>
{/if}
