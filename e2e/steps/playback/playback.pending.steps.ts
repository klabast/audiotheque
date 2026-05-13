/**
 * Pending step definitions for features that are not yet implemented.
 * These return 'pending' to avoid timeouts on undefined steps.
 * As features are built, move steps to the appropriate given/when/then files.
 */
import { Given, When, Then } from '@cucumber/cucumber';
import { AudiodWorld } from '../../support/world';

// =====================
// Queue management (not yet implemented)
// =====================

When('User adds {string} to queue', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

When('User adds track {string} to queue', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

When('User adds album {string} to queue', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

When('User adds {string} to play next', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

When('User adds track {string} to play next', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

When('User adds album {string} to play next', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

When('User opens queue view', async function (this: AudiodWorld) {
	return 'pending';
});

When('User removes {string} from queue', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

When('User clears queue', async function (this: AudiodWorld) {
	return 'pending';
});

When('User moves {string} to position {int}', async function (this: AudiodWorld, _track: string, _pos: number) {
	return 'pending';
});

When('User plays {string} from queue', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

When('{string} finishes playing', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

When('Current track finishes', async function (this: AudiodWorld) {
	return 'pending';
});

When('User starts playing album {string}', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

When('User confirms playing album {string}', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

When('User cancels playing album {string}', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

When('User restores previous session', async function (this: AudiodWorld) {
	return 'pending';
});

When('User enables shuffle', async function (this: AudiodWorld) {
	return 'pending';
});

When('Last track of source finishes', async function (this: AudiodWorld) {
	return 'pending';
});

Given('User has tracks in queue', async function (this: AudiodWorld) {
	return 'pending';
});

Given('User has added {string} to queue', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Given('User has added {string} to play next', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Given('Queue contains {string} and {string}', async function (this: AudiodWorld, _t1: string, _t2: string) {
	return 'pending';
});

Given('Queue contains {string}, {string}, {string}', async function (this: AudiodWorld, _t1: string, _t2: string, _t3: string) {
	return 'pending';
});

Given('Queue contains {string} from album {string}', async function (this: AudiodWorld, _track: string, _album: string) {
	return 'pending';
});

Given('Queue contains multiple tracks', async function (this: AudiodWorld) {
	return 'pending';
});

Given('User is playing album {string} from track {int}', async function (this: AudiodWorld, _album: string, _track: number) {
	return 'pending';
});

Given('Queue is empty', async function (this: AudiodWorld) {
	return 'pending';
});

Given('User has previous session with album {string}', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

Given('Repeat is enabled', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Queue contains {string} at last position', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Then('Source remains album {string}', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

Then('Current track is {string} from source', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Then('Queue order is {string}, {string}', async function (this: AudiodWorld, _t1: string, _t2: string) {
	return 'pending';
});

Then('Queue order is {string}, {string}, {string}', async function (this: AudiodWorld, _t1: string, _t2: string, _t3: string) {
	return 'pending';
});

Then('Queue contains {string} at position {int}', async function (this: AudiodWorld, _track: string, _pos: number) {
	return 'pending';
});

Then('Queue contains {string}', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Then('User remains on album details page', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Queue contains all tracks from {string} at end', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

Then('Queue contains all tracks from {string}', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

Then('{string} plays after current track', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Then('Next track is {string} from source', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Then('Queue view shows explicit queue section', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Queue view shows source section', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Queue view shows playing from {string}', async function (this: AudiodWorld, _album: string) {
	return 'pending';
});

Then('Queue does not contain {string}', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Then('Queue shows {string} with source {string}', async function (this: AudiodWorld, _track: string, _source: string) {
	return 'pending';
});

Then('{string} is removed from queue', async function (this: AudiodWorld, _track: string) {
	return 'pending';
});

Then('Source shows remaining tracks', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Current track is next track from source', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Source remaining count decreases', async function (this: AudiodWorld) {
	return 'pending';
});

Then('User is warned about replacing current session', async function (this: AudiodWorld) {
	return 'pending';
});

Then('User can confirm or cancel', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Queue is unchanged', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Previous session is saved to history', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Queue is restored from previous session', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Source tracks are in random order', async function (this: AudiodWorld) {
	return 'pending';
});

Then('All tracks will eventually play', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Playback continues from first track', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Source is reset to full album', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Music continues playing', async function (this: AudiodWorld) {
	return 'pending';
});

// =====================
// MPD discovery (not yet implemented)
// =====================

Given('MPD device {string} is on the network', async function (this: AudiodWorld, _device: string) {
	return 'pending';
});

Given('MPD device {string} is offline', async function (this: AudiodWorld, _device: string) {
	return 'pending';
});

Given('MPD device {string} is discovered', async function (this: AudiodWorld, _device: string) {
	return 'pending';
});

Given('No MPD devices are discovered', async function (this: AudiodWorld) {
	return 'pending';
});

When('{string} goes offline', async function (this: AudiodWorld, _device: string) {
	return 'pending';
});

When('{string} comes online', async function (this: AudiodWorld, _device: string) {
	return 'pending';
});

When('User adds MPD device manually with host {string} port {string}', async function (this: AudiodWorld, _host: string, _port: string) {
	return 'pending';
});

When('User renames device to {string}', async function (this: AudiodWorld, _name: string) {
	return 'pending';
});

Then('Device list shows {string} as unavailable', async function (this: AudiodWorld, _device: string) {
	return 'pending';
});

Then('Device list shows {string} as available', async function (this: AudiodWorld, _device: string) {
	return 'pending';
});

Then('Device list shows manually configured device', async function (this: AudiodWorld) {
	return 'pending';
});

Then('Device can be used for playback', async function (this: AudiodWorld) {
	return 'pending';
});

// Note: Multi-browser "Browser A/B shows {string}" steps are implemented
// in playback.then.steps.ts — they are no longer pending.

// Note: "No music is playing" and "Queue is empty" Given steps are already defined
// in playback.given.steps.ts — do not duplicate them here.
