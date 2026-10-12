/**
 * Queue steps for playback - Given
 */
import { Given } from '@cucumber/cucumber';
import { AudiodWorld } from '../../support/world';
import { setQueue } from '../../support/queue';

Given('Queue contains {string}', async function (this: AudiodWorld, track: string) {
	await setQueue(this.getPage(), [track]);
});

Given('Queue contains {string}, {string}', async function (this: AudiodWorld, t1: string, t2: string) {
	await setQueue(this.getPage(), [t1, t2]);
});

Given(
	'Queue contains {string}, {string}, {string}',
	async function (this: AudiodWorld, t1: string, t2: string, t3: string) {
		await setQueue(this.getPage(), [t1, t2, t3]);
	}
);
