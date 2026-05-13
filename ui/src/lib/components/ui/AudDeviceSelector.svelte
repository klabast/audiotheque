<script lang="ts">
	import { DropdownMenu } from 'bits-ui';
	import type { DeviceInfo } from '$lib/services/api';

	interface Props {
		deviceName: string;
		isRemote?: boolean;
		variant?: 'dropdown' | 'card';
		devices?: DeviceInfo[];
		currentDeviceId?: string;
		onDeviceSelect?: (deviceId: string) => void;
		playingOnLabel?: string;
		thisDeviceLabel?: string;
		selectDeviceLabel?: string;
	}

	let {
		deviceName,
		isRemote = false,
		variant = 'dropdown',
		devices = [],
		currentDeviceId = '',
		onDeviceSelect,
		playingOnLabel = 'Playing on',
		thisDeviceLabel = 'This Device',
		selectDeviceLabel = 'Select device'
	}: Props = $props();

	function getDeviceIcon(type: string): string {
		return type === 'mpd' ? 'i-lucide-speaker' : 'i-lucide-smartphone';
	}
</script>

{#if variant === 'dropdown'}
	<!-- Desktop dropdown style -->
	<DropdownMenu.Root>
		<DropdownMenu.Trigger
			class="device-selector flex items-center gap-1.5 text-xs min-w-0 transition-colors {isRemote
				? 'text-primary hover:text-primary-muted'
				: 'text-text-muted hover:text-text-primary'}"
			aria-label={selectDeviceLabel}
			data-testid="device-picker-button"
		>
			<div class="i-lucide-speaker w-4 h-4 flex-shrink-0"></div>
			<span class="truncate max-w-[100px]" data-testid={isRemote ? 'device-indicator' : undefined}
				>{deviceName}</span
			>
			<div class="i-lucide-chevron-down w-3 h-3 flex-shrink-0 opacity-60"></div>
		</DropdownMenu.Trigger>

		<DropdownMenu.Content
			class="bg-surface border-surface-hover z-50 min-w-[12rem] rounded-lg border p-1 shadow-lg"
			side="top"
			align="end"
			data-testid="device-picker-menu"
		>
			{#each devices as device (device.id)}
				{@const isActive = device.id === currentDeviceId}
				<DropdownMenu.Item
					class="flex items-center gap-2 cursor-pointer rounded px-3 py-2 text-sm transition-colors focus:outline-none {isActive
						? 'text-primary bg-primary/5'
						: 'text-text-primary hover:bg-surface-hover focus:bg-surface-hover'}"
					onSelect={() => onDeviceSelect?.(device.id)}
					data-testid={device.isCurrent
						? 'device-option-this-browser'
						: `device-option-${device.id}`}
				>
					<div class="{getDeviceIcon(device.type)} w-4 h-4 flex-shrink-0"></div>
					<span class="flex-1 truncate">{device.isCurrent ? thisDeviceLabel : device.name}</span>
					{#if isActive}
						<div class="i-lucide-check w-4 h-4 flex-shrink-0"></div>
					{/if}
				</DropdownMenu.Item>
			{/each}
		</DropdownMenu.Content>
	</DropdownMenu.Root>
{:else}
	<!-- Mobile card style -->
	<DropdownMenu.Root>
		<DropdownMenu.Trigger
			class="w-full flex items-center gap-3 p-3 rounded-xl active:bg-surface-hover transition-colors {isRemote
				? 'bg-surface border border-primary'
				: 'bg-surface'}"
			aria-label={selectDeviceLabel}
			data-testid="device-picker-button"
		>
			<div
				class="w-9 h-9 flex items-center justify-center rounded-full {isRemote
					? 'bg-primary text-white'
					: 'bg-surface-hover text-text-secondary'}"
			>
				<div class="i-lucide-smartphone w-5 h-5"></div>
			</div>
			<div class="flex-1 text-left min-w-0">
				<div class="text-xs text-text-muted">{playingOnLabel}</div>
				<div class="text-sm font-medium truncate {isRemote ? 'text-primary' : 'text-text-primary'}">
					{deviceName}
				</div>
			</div>
			<div class="i-lucide-chevron-down w-5 h-5 text-text-muted"></div>
		</DropdownMenu.Trigger>

		<DropdownMenu.Content
			class="bg-surface border-surface-hover z-50 min-w-[14rem] rounded-lg border p-1 shadow-lg"
			side="top"
			data-testid="device-picker-menu"
		>
			{#each devices as device (device.id)}
				{@const isActive = device.id === currentDeviceId}
				<DropdownMenu.Item
					class="flex items-center gap-2 cursor-pointer rounded px-3 py-2 text-sm transition-colors focus:outline-none {isActive
						? 'text-primary bg-primary/5'
						: 'text-text-primary hover:bg-surface-hover focus:bg-surface-hover'}"
					onSelect={() => onDeviceSelect?.(device.id)}
					data-testid={device.isCurrent
						? 'device-option-this-browser'
						: `device-option-${device.id}`}
				>
					<div class="{getDeviceIcon(device.type)} w-4 h-4 flex-shrink-0"></div>
					<span class="flex-1 truncate">{device.isCurrent ? thisDeviceLabel : device.name}</span>
					{#if isActive}
						<div class="i-lucide-check w-4 h-4 flex-shrink-0"></div>
					{/if}
				</DropdownMenu.Item>
			{/each}
		</DropdownMenu.Content>
	</DropdownMenu.Root>
{/if}
