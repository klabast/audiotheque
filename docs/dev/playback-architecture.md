# Playback Architecture

## Core Principle

**HTMLAudioElement = Source of Truth for playback state**

- Store manages SESSION state (what to play): current track, queue, source
- Audio element owns PLAYBACK state (how it's playing): currentTime, paused, volume
- Svelte bindings sync UI automatically - no manual event listeners, no state duplication

## Architecture Overview

```
playback.svelte.ts (ONE store)
├── session: { currentTrack, queue, remaining, source }  ← from API
├── audioElement: HTMLAudioElement reference              ← set by PlayFooter
└── methods: playAlbum(), next(), prev(), play(), pause(), seek()

PlayFooter.svelte (audio element host)
├── <audio bind:currentTime bind:paused bind:duration bind:volume>
├── Controls (play/pause, next, prev, seek slider)
└── Track info display
```

## Component Hierarchy

```
+layout.svelte (authenticated)
└── AppLayout.svelte
    ├── TopBar.svelte
    ├── <slot /> (page content - changes on navigation)
    └── PlayFooter.svelte
        └── <audio> element (PERSISTENT - survives navigation)
```

## State Separation

| State Type | Owner | Examples |
|------------|-------|----------|
| Session State | Store (from API) | currentTrack, queue, source, remaining |
| Playback State | Audio Element | currentTime, duration, paused, volume |

**Why this matters:** v1 had state scattered everywhere, trying to sync store state with audio element state. This caused bugs. Now the audio element IS the state - we just bind to it.

## State Flows

### Play Album
```
User clicks play → playback.playAlbum(id)
→ POST /api/playback/play → session updated in store
→ trackUrl derived changes → audio src updates
→ audio plays → bound state updates UI automatically
```

### Track Ends
```
Audio 'ended' event → playback.next()
→ POST /api/playback/next → new track in session
→ audio src updates → plays new track
```

### App Load (Session Restore)
```
PlayFooter mounts → playback.loadSession()
→ GET /api/playback/session → session populated
→ trackUrl set → audio ready (paused, requires user gesture)
```

### User Seeks
```
User drags slider → playback.seek(time)
→ audioElement.currentTime = time
→ audio seeks → bound currentTime updates slider
```

## API Endpoints

### Track Streaming
```
GET /api/tracks/{id}/stream
- Authenticates user
- Verifies user has access to track's library
- Serves audio file with proper Content-Type
- Supports range requests (for seeking)
```

### Playback Control
```
POST /api/playback/play      - Start playing album/track
GET  /api/playback/session   - Get current session
POST /api/playback/next      - Advance to next track
POST /api/playback/previous  - Go to previous track
POST /api/playback/pause     - Pause and save position
POST /api/playback/resume    - Resume playback
```

## Store Structure

```typescript
// ui/src/lib/stores/playback.svelte.ts

interface TrackInfo {
    id: number;
    title: string;
    artistName: string;
    albumId: number;
    duration: number; // milliseconds
}

interface PlaybackSession {
    source: { type: string; id: number } | null;
    currentTrack: TrackInfo | null;
    queue: number[];
    remaining: number[];
}

export const playback = (() => {
    // SESSION STATE (from API)
    let session = $state<PlaybackSession>({ ... });

    // AUDIO ELEMENT REFERENCE (set by PlayFooter)
    let audioElement = $state<HTMLAudioElement | null>(null);

    // DERIVED
    const hasSession = $derived(session.currentTrack !== null);
    const trackUrl = $derived(
        session.currentTrack ? `/api/tracks/${session.currentTrack.id}/stream` : null
    );

    return {
        // Getters
        get session() { return session; },
        get hasSession() { return hasSession; },
        get trackUrl() { return trackUrl; },

        // Audio element management
        setAudioElement(el) { audioElement = el; },

        // Session actions (API calls)
        async playAlbum(albumId) { ... },
        async loadSession() { ... },
        async next() { ... },
        async previous() { ... },

        // Audio controls (direct element manipulation)
        play() { audioElement?.play(); },
        pause() { audioElement?.pause(); },
        seek(time) { if (audioElement) audioElement.currentTime = time; }
    };
})();
```

## PlayFooter Structure

```svelte
<script lang="ts">
    import { playback } from '$lib/stores/playback.svelte';
    import { onMount } from 'svelte';

    // Bound to audio element (NOT in store - avoids duplication)
    let currentTime = $state(0);
    let duration = $state(0);
    let paused = $state(true);
    let volume = $state(1);
    let audioRef: HTMLAudioElement;

    onMount(() => {
        playback.setAudioElement(audioRef);
        playback.loadSession();
        return () => playback.setAudioElement(null);
    });
</script>

<audio
    bind:this={audioRef}
    bind:currentTime
    bind:duration
    bind:paused
    bind:volume
    src={playback.trackUrl}
    on:ended={() => playback.next()}
/>

<footer data-testid="player-footer">
    <!-- Controls use local bound state for display -->
    <!-- Controls call playback.play(), playback.pause(), etc. for actions -->
</footer>
```

## Key Constraints

1. **NO state duplication** - currentTime, paused, volume come from audio element bindings only
2. **ONE audio element** - Lives in PlayFooter, reference stored in playback store
3. **Session vs Playback** - Session = what to play (API), Playback = how it's playing (DOM)
4. **Persistence** - Audio element in AppLayout survives route navigation
