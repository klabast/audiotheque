<script lang="ts">
	interface Props {
		variant?: 'default' | 'muted' | 'mobile' | 'mobile-lg' | 'mobile-play';
		icon: string;
		label: string;
		onclick?: () => void;
		testId?: string;
	}

	let { variant = 'default', icon, label, onclick, testId }: Props = $props();

	const variantClasses: Record<string, string> = {
		default: 'player-btn',
		muted: 'player-btn-muted',
		mobile: 'player-btn-mobile',
		'mobile-lg': 'player-btn-mobile-lg',
		'mobile-play': 'player-btn-mobile-play'
	};
</script>

<button class={variantClasses[variant]} {onclick} aria-label={label} data-testid={testId}>
	<div
		class="{icon} {variant === 'mobile-play'
			? 'w-9 h-9'
			: variant === 'mobile-lg'
				? 'w-8 h-8'
				: variant === 'mobile'
					? 'w-6 h-6'
					: 'w-5 h-5'}"
	></div>
</button>
