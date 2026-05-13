<script lang="ts">
	import type { Snippet } from 'svelte';
	import TopBar from './TopBar.svelte';
	import LeftDrawer from './LeftDrawer.svelte';
	import RightSidebar from './RightSidebar.svelte';
	import PlayFooter from './PlayFooter.svelte';
	import QueuePanel from '$lib/components/QueuePanel.svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	let isLeftDrawerOpen = $state(false);
	let isRightSidebarOpen = $state(false);

	function toggleLeftDrawer() {
		isLeftDrawerOpen = !isLeftDrawerOpen;
	}

	function closeLeftDrawer() {
		isLeftDrawerOpen = false;
	}

	function toggleRightSidebar() {
		isRightSidebarOpen = !isRightSidebarOpen;
	}

	function closeRightSidebar() {
		isRightSidebarOpen = false;
	}
</script>

<!-- Main Layout: Full viewport with header/main -->
<div class="bg-background flex h-screen flex-col">
	<!-- Header -->
	<TopBar {toggleLeftDrawer} {toggleRightSidebar} />

	<!-- Main Content Area with parallax effect - fills remaining space -->
	<div class="relative flex-1 overflow-hidden">
		<!-- Left Drawer (slides in from left) -->
		<LeftDrawer {isLeftDrawerOpen} {closeLeftDrawer} />

		<!-- Main Content Wrapper (shifts right with parallax and dims) -->
		<div
			class="h-full transition-all duration-300 ease-out {isLeftDrawerOpen
				? 'translate-x-32'
				: 'translate-x-0'}"
		>
			<!-- Dim overlay when drawer is open -->
			{#if isLeftDrawerOpen}
				<div
					class="fixed inset-0 z-30 bg-black/50 backdrop-blur-sm"
					onclick={closeLeftDrawer}
					onkeydown={(e) => e.key === 'Escape' && closeLeftDrawer()}
					role="button"
					tabindex="-1"
					aria-label="Close drawer"
				></div>
			{/if}

			<!-- Main Content (always visible, scrollable) -->
			<main class="bg-background h-full overflow-auto p-4">
				{@render children()}
			</main>

			<!-- Right Sidebar (overlay drawer at all sizes) -->
			<RightSidebar {isRightSidebarOpen} {closeRightSidebar}>
				<QueuePanel />
			</RightSidebar>
		</div>
	</div>

	<!-- Footer (in flex flow, takes natural height) -->
	<PlayFooter />
</div>
