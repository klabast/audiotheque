import { describe, expect, it } from 'vitest';
import { render, screen, within } from '@testing-library/svelte';
import { tick } from 'svelte';
import '@testing-library/jest-dom/vitest';
import AudPlayFooter from './AudPlayFooter.svelte';

const baseProps = {
	trackTitle: 'Some Song',
	trackArtist: 'Some Artist',
	trackAlbum: 'Some Album',
	albumCover: null,
	currentTime: 0,
	duration: 200,
	paused: false,
	volume: 0.8,
	muted: false,
	supportsVolume: true,
	deviceName: 'Living Room',
	isRemoteDevice: true,
	visible: true,
	isFullScreenOpen: true,
	onPlayPause: () => {},
	onPrevious: () => {},
	onNext: () => {},
	onSeek: () => {},
	onVolumeChange: () => {},
	onToggleMute: () => {}
};

describe('AudPlayFooter — fullscreen detail view exposes the device picker', () => {
	it('renders the device picker in the fullscreen overlay even when the local devices cache is empty', async () => {
		// Repro of the user-reported bug: a remote device is active and shown in
		// the footer, so the user taps to open fullscreen to switch back. If the
		// `devices` array is briefly empty (WS welcome race, refetch in flight,
		// or any later staleness), the previous gate on `showDeviceSelector`
		// hid the picker entirely — leaving the user stranded on the remote
		// device with no way to change playback. The fullscreen picker must
		// always be present when the overlay is open.
		render(AudPlayFooter, {
			props: {
				...baseProps,
				// Parent normally passes `showDeviceSelector = playback.hasDevices`,
				// which evaluates to false for an empty list. Mirror that here.
				showDeviceSelector: false,
				devices: [],
				currentDeviceId: 'mpd-living-room',
				onDeviceSelect: () => {}
			}
		});
		await tick();

		const fullscreen = screen.getByTestId('player-fullscreen');
		expect(within(fullscreen).queryByTestId('device-picker-button')).toBeInTheDocument();
	});

	it('still hides the device picker from the desktop footer bar when no devices are known', async () => {
		// The desktop row crams volume + device picker into a tight right column.
		// When there are no devices to choose from, the desktop picker stays
		// hidden to avoid an empty dropdown taking up space next to the volume
		// slider. Only the mobile fullscreen overlay is unconditional.
		render(AudPlayFooter, {
			props: {
				...baseProps,
				isFullScreenOpen: false,
				showDeviceSelector: false,
				devices: [],
				currentDeviceId: '',
				onDeviceSelect: () => {}
			}
		});
		await tick();

		const footer = screen.getByTestId('player-footer');
		expect(within(footer).queryByTestId('device-picker-button')).not.toBeInTheDocument();
	});
});
