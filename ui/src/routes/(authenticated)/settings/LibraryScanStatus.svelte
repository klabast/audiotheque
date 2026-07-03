<script lang="ts">
	import { Alert } from '$lib/components/ui';
	import type { ScanProgressData } from '$lib/services/ws-client';

	interface Props {
		libraryId: number;
		scanError?: string;
		progress?: ScanProgressData;
	}

	let { libraryId, scanError, progress }: Props = $props();
</script>

{#if scanError}
	<div class="mt-4">
		<Alert variant="error" data-testid="scan-error-message-{libraryId}">
			Error: {scanError}
		</Alert>
	</div>
{/if}

{#if progress?.status === 'running'}
	{@const percentage =
		progress.totalFiles > 0 ? Math.round((progress.processedFiles / progress.totalFiles) * 100) : 0}
	<div class="mt-4" data-testid="scan-progress-bar-{libraryId}">
		<div class="flex-between mb-2 text-sm">
			<span class="text-text-secondary" data-testid="scan-progress-file-count-{libraryId}">
				{progress.processedFiles} / {progress.totalFiles} files
			</span>
			<span
				class="text-text-primary font-medium"
				data-testid="scan-progress-percentage-{libraryId}"
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
				data-testid="scan-progress-current-file-{libraryId}"
			>
				{progress.currentFile.split('/').pop()}
			</div>
		{/if}
		<div
			class="text-text-secondary mt-2 flex gap-3 text-xs"
			data-testid="scan-progress-stats-{libraryId}"
		>
			<span>Added: {progress.tracksAdded}</span>
			<span>Updated: {progress.tracksUpdated}</span>
			<span>Errors: {progress.errors}</span>
		</div>
	</div>
{/if}

{#if progress?.status === 'completed'}
	<div class="mt-4">
		<Alert variant="success" data-testid="library-scan-complete">
			✓ Scan completed: {progress.tracksAdded} tracks added, {progress.tracksUpdated} updated
		</Alert>
	</div>
{/if}
