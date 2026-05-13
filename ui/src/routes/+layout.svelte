<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/stores';
	import '../app.css';
	import 'virtual:uno.css';

	let { data, children } = $props();

	// Redirect to init if required (unless already on init page)
	$effect(() => {
		if (data.setupRequired && $page.url.pathname !== '/init') {
			goto('/init');
		} else if (!data.setupRequired && $page.url.pathname === '/init') {
			// If trying to access init when not required, redirect to login
			goto('/login');
		}
	});
</script>

{@render children()}
