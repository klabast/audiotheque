import { describe, expect, it } from 'vitest';
import { render, fireEvent, screen } from '@testing-library/svelte';
import { tick } from 'svelte';
import '@testing-library/jest-dom/vitest';
import AudDeviceSelector from './AudDeviceSelector.svelte';
import type { DeviceInfo } from '$lib/services/api';

describe('AudDeviceSelector — multi-browser labelling', () => {
	it('renders "This Device" for the current browser and the real name for other browsers', async () => {
		// Two browser tabs are connected as devices. The server has marked c1 as
		// the current one (isCurrent=true) — i.e. the tab making this request.
		// The other tab (c2) carries its UA-derived name and must NOT be labelled
		// "This Device" just because it is a browser.
		const devices: DeviceInfo[] = [
			{ id: 'c1', name: 'Chrome on macOS', type: 'browser', isCurrent: true },
			{ id: 'c2', name: 'Firefox on Linux', type: 'browser', isCurrent: false }
		];

		render(AudDeviceSelector, {
			props: {
				deviceName: 'Chrome on macOS',
				devices,
				currentDeviceId: 'c1',
				thisDeviceLabel: 'This Device'
			}
		});
		await tick();

		// Open the dropdown so its content is in the DOM.
		await fireEvent.click(screen.getByTestId('device-picker-button'));
		await tick();

		// The current browser tab is labelled with a stable testid (the
		// X-Audiod-Client-Id is server-assigned and unknowable from outside the
		// hub), while other browsers/MPDs keep `device-option-<id>`.
		const currentOption = await screen.findByTestId('device-option-this-browser');
		const otherOption = await screen.findByTestId('device-option-c2');

		expect(currentOption).toHaveTextContent('This Device');
		expect(otherOption).toHaveTextContent('Firefox on Linux');
		expect(otherOption).not.toHaveTextContent('This Device');
	});
});
