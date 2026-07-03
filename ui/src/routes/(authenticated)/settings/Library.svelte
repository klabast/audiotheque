<script lang="ts">
	import { onDestroy, onMount } from 'svelte';
	import { SvelteMap } from 'svelte/reactivity';
	import { auth } from '$lib/stores/auth.svelte';
	import { api } from '$lib/services/api';
	import { scan } from '$lib/stores/scan.svelte';
	import { Button, ConfirmDeleteModal } from '$lib/components/ui';
	import type { LibraryLibraryResponse } from '$lib/api/generated/src';
	import * as m from '$lib/paraglide/messages';
	import LibraryCreateForm from './LibraryCreateForm.svelte';
	import LibraryItem from './LibraryItem.svelte';
	import LibraryEditModal from './LibraryEditModal.svelte';

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
	let libraryToEdit: LibraryWithStats | null = $state(null);

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
	}

	async function handleCreate(name: string, paths: string[]) {
		await api.createLibrary(name, paths);

		// The new library auto-scans on creation; progress flows through
		// the global scan store via the app-wide WebSocket subscription,
		// and library-updated events trigger refreshes via onLibraryUpdated.

		// Close form before the list refresh so the user isn't stuck on it
		showForm = false;
		libraries = (await api.listLibraries()) as LibraryWithStats[];
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
	}

	function closeEditModal() {
		libraryToEdit = null;
	}

	async function handleUpdate(name: string, paths: string[]) {
		if (!libraryToEdit) return;

		await api.updateLibrary(libraryToEdit.id, name, paths);
		// Close the modal as soon as the update has succeeded; refreshing the
		// list is independent and shouldn't keep the user staring at the form.
		closeEditModal();
		libraries = (await api.listLibraries()) as LibraryWithStats[];
	}
</script>

<div class="space-y-6">
	<div>
		<h2 class="text-text-primary text-2xl font-bold">Library Settings</h2>
		<p class="text-text-secondary mt-1 text-sm">Manage your music libraries</p>
	</div>

	{#if showForm}
		<LibraryCreateForm onSubmit={handleCreate} onCancel={toggleForm} />
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
				<LibraryItem
					{library}
					isAdmin={!!auth.user?.isAdmin}
					scanError={scanErrors.get(library.id)}
					scanProgress={scanProgress.get(library.id)}
					onScan={handleScan}
					onEdit={openEditModal}
					onDelete={openDeleteModal}
				/>
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
{#if libraryToEdit}
	<LibraryEditModal library={libraryToEdit} onSave={handleUpdate} onCancel={closeEditModal} />
{/if}
