<script lang="ts">
	import { api, type GetStatsPeriodEnum, type PlaysStatsResponse } from '$lib/services/api';
	import * as m from '$lib/paraglide/messages';

	const PERIODS: { value: GetStatsPeriodEnum; label: () => string }[] = [
		{ value: 'week', label: m['stats.period.week'] },
		{ value: 'month', label: m['stats.period.month'] },
		{ value: 'year', label: m['stats.period.year'] },
		{ value: 'all', label: m['stats.period.all'] }
	];

	let period = $state<GetStatsPeriodEnum>('month');
	let stats = $state<PlaysStatsResponse | null>(null);
	let failed = $state(false);

	$effect(() => {
		load(period);
	});

	async function load(p: GetStatsPeriodEnum) {
		failed = false;
		try {
			const result = await api.getStats(p);
			if (p === period) stats = result;
		} catch (err) {
			console.error('[stats] load failed', err);
			failed = true;
		}
	}

	function listeningTime(seconds: number): string {
		const minutes = Math.floor(seconds / 60);
		if (minutes < 60) return m['stats.minutes']({ minutes });
		return m['stats.hours_minutes']({ hours: Math.floor(minutes / 60), minutes: minutes % 60 });
	}

	const topTracks = $derived(stats?.topTracks ?? []);
	const topAlbums = $derived(stats?.topAlbums ?? []);
	const topArtists = $derived(stats?.topArtists ?? []);
</script>

<div class="mx-auto flex max-w-5xl flex-col gap-6 p-4 md:p-6">
	<div class="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
		<h1 class="text-text-primary text-2xl font-bold">{m['stats.title']()}</h1>
		<div class="bg-surface-hover flex gap-1 self-start rounded-full p-1">
			{#each PERIODS as p (p.value)}
				<button
					type="button"
					onclick={() => (period = p.value)}
					aria-pressed={period === p.value}
					class="rounded-full px-3 py-1 text-sm transition-colors {period === p.value
						? 'bg-surface text-text-primary shadow-sm'
						: 'text-text-secondary hover:text-text-primary'}"
					data-testid="stats-period-{p.value}-button"
				>
					{p.label()}
				</button>
			{/each}
		</div>
	</div>

	{#if failed}
		<p class="text-error" data-testid="stats-error">{m['stats.load_failed']()}</p>
	{:else if stats}
		<div class="grid grid-cols-2 gap-4">
			<div class="bg-surface rounded-lg p-4">
				<p class="text-text-secondary text-xs uppercase">{m['stats.listens']()}</p>
				<p class="text-text-primary text-2xl font-bold" data-testid="stats-listens">
					{stats.listens ?? 0}
				</p>
			</div>
			<div class="bg-surface rounded-lg p-4">
				<p class="text-text-secondary text-xs uppercase">{m['stats.listening_time']()}</p>
				<p class="text-text-primary text-2xl font-bold" data-testid="stats-listening-time">
					{listeningTime(stats.listeningSeconds ?? 0)}
				</p>
			</div>
		</div>

		{#if !stats.listens}
			<p class="text-text-secondary py-8 text-center" data-testid="stats-empty">
				{m['stats.empty']()}
			</p>
		{:else}
			<div class="grid gap-6 lg:grid-cols-2">
				<section class="min-w-0 lg:col-span-2" data-testid="stats-top-tracks">
					<h2 class="text-text-primary mb-2 text-lg font-semibold">{m['stats.top_tracks']()}</h2>
					<ol class="flex flex-col">
						{#each topTracks as track, i (track.trackId)}
							<li data-testid="stats-top-track-{track.trackId}">
								<a
									href="/album/{track.albumId}"
									class="hover:bg-surface-hover flex items-center gap-3 rounded p-2"
								>
									<span class="text-text-secondary w-6 text-right text-sm">{i + 1}</span>
									<img
										src="/api/albums/{track.albumId}/cover"
										alt=""
										class="bg-surface-hover h-10 w-10 flex-shrink-0 rounded object-cover"
										onerror={(e) => ((e.target as HTMLImageElement).style.visibility = 'hidden')}
									/>
									<span class="min-w-0 flex-1">
										<span class="text-text-primary block truncate text-sm">{track.title}</span>
										<span class="text-text-secondary block truncate text-xs">
											{track.artistName} · {track.albumTitle}
										</span>
									</span>
									<span
										class="text-text-secondary text-sm"
										title={m['stats.plays']()}
										data-testid="stats-top-track-plays"
									>
										{track.plays}
									</span>
								</a>
							</li>
						{/each}
					</ol>
				</section>

				<section class="min-w-0" data-testid="stats-top-albums">
					<h2 class="text-text-primary mb-2 text-lg font-semibold">{m['stats.top_albums']()}</h2>
					<ol class="flex flex-col">
						{#each topAlbums as album, i (album.albumId)}
							<li data-testid="stats-top-album-{album.albumId}">
								<a
									href="/album/{album.albumId}"
									class="hover:bg-surface-hover flex items-center gap-3 rounded p-2"
								>
									<span class="text-text-secondary w-6 text-right text-sm">{i + 1}</span>
									<img
										src="/api/albums/{album.albumId}/cover"
										alt=""
										class="bg-surface-hover h-10 w-10 flex-shrink-0 rounded object-cover"
										onerror={(e) => ((e.target as HTMLImageElement).style.visibility = 'hidden')}
									/>
									<span class="min-w-0 flex-1">
										<span class="text-text-primary block truncate text-sm">{album.title}</span>
										<span class="text-text-secondary block truncate text-xs">
											{album.artistName}
										</span>
									</span>
									<span class="text-text-secondary text-sm" title={m['stats.plays']()}>
										{album.plays}
									</span>
								</a>
							</li>
						{/each}
					</ol>
				</section>

				<section class="min-w-0" data-testid="stats-top-artists">
					<h2 class="text-text-primary mb-2 text-lg font-semibold">
						{m['stats.top_artists']()}
					</h2>
					<ol class="flex flex-col">
						{#each topArtists as artist, i (artist.artistId)}
							<li
								class="flex items-center gap-3 p-2"
								data-testid="stats-top-artist-{artist.artistId}"
							>
								<span class="text-text-secondary w-6 text-right text-sm">{i + 1}</span>
								<span class="text-text-primary min-w-0 flex-1 truncate text-sm">{artist.name}</span>
								<span class="text-text-secondary text-sm" title={m['stats.plays']()}>
									{artist.plays}
								</span>
							</li>
						{/each}
					</ol>
				</section>
			</div>
		{/if}
	{/if}
</div>
