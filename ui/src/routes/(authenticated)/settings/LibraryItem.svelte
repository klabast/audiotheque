<script lang="ts">
	import { Button, DropdownButton } from '$lib/components/ui';
	import { Pencil, RefreshCw, RotateCcw, Trash2 } from 'lucide-svelte';
	import type { LibraryLibraryResponse } from '$lib/api/generated/src';
	import type { ScanProgressData } from '$lib/services/ws-client';
	import LibraryScanStatus from './LibraryScanStatus.svelte';

	interface LibraryWithStats extends LibraryLibraryResponse {
		id: number;
		name: string;
		paths: string[];
		trackCount?: number;
		albumCount?: number;
		lastScanTime?: string;
	}

	interface Props {
		library: LibraryWithStats;
		isAdmin: boolean;
		scanError?: string;
		scanProgress?: ScanProgressData;
		onScan: (libraryId: number) => void;
		onEdit: (library: LibraryWithStats) => void;
		onDelete: (library: LibraryWithStats) => void;
	}

	let { library, isAdmin, scanError, scanProgress, onScan, onEdit, onDelete }: Props = $props();
</script>

<div class="card-bordered" data-testid="library-item-{library.name}">
	<div class="flex items-start justify-between">
		<div class="flex-1">
			<h3 class="text-text-primary text-lg font-semibold">{library.name}</h3>

			<div
				class="text-text-secondary mt-2 space-y-1 text-sm"
				data-testid="library-paths-{library.id}"
			>
				{#each library.paths as path, index (path)}
					<div class="flex items-center gap-2" data-testid="library-path-{library.id}-{index}">
						<span>📁</span>
						<span data-testid="library-path-value-{library.id}-{index}">{path}</span>
					</div>
				{/each}
			</div>

			<div class="text-text-secondary mt-3 flex gap-4 text-sm">
				<span data-testid="library-track-count-{library.id}">{library.trackCount} tracks</span>
				<span data-testid="library-album-count-{library.id}">{library.albumCount} albums</span>
				{#if library.lastScanTime}
					<span>Last scanned: {new Date(library.lastScanTime).toLocaleString()}</span>
				{/if}
			</div>

			<LibraryScanStatus libraryId={library.id} {scanError} progress={scanProgress} />
		</div>

		{#if isAdmin}
			<div class="flex gap-2">
				<Button
					type="button"
					variant="ghost"
					data-testid="edit-library-button-{library.id}"
					onclick={() => onEdit(library)}
					class="px-3"
				>
					<Pencil class="h-4 w-4" />
				</Button>

				<DropdownButton
					label="Scan"
					icon={RefreshCw}
					variant="ghost"
					onMainClick={() => onScan(library.id)}
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
					onclick={() => onDelete(library)}
					class="px-3"
				>
					<Trash2 class="h-4 w-4" />
				</Button>
			</div>
		{/if}
	</div>
</div>
