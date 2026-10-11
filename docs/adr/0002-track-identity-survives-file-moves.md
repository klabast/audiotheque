# 2. Track identity survives file moves

Date: 2026-10-11

## Status

Proposed

## Context

Favorites, playlists, a persistent queue and play history are next on the
roadmap. All of them hold references to tracks and albums. Today those
references would not survive normal library maintenance.

**A track is its file path.** `track` is unique on `(library_id, file_path)`.
The scanner looks a file up by path (`CreateOrUpdateTrack`), and at the end of
a scan deletes every row whose path it didn't see (`removeVanishedTracks` →
`DeleteTracksByPaths`). Rename a file, move a folder, or let Picard rename
while retagging, and the track is deleted and re-created with a new id.
Everything pointing at the old id goes with it.

**An album is its folder.** `GetOrCreateAlbum` dedups on
`(library_id, artist_id, title, folder_path)` (migration 00007, so two rips of
the same release in different folders stay apart). Moving the folder creates a
second album. The old one is never deleted, because nothing in the scanner
deletes albums, so it lingers with no tracks.

Artists are keyed on `(library_id, name)` and are already stable across moves.

The library is read-only to us: we never write to the user's files.

Things that happen to a real library, and what should happen to the identity:

| change | path | bytes | tags | should keep id |
|---|---|---|---|---|
| retag in place | same | changed | changed | yes (works today) |
| rename / move | changed | same | same | yes |
| Picard retag + rename | changed | changed | changed | yes |
| replace MP3 with FLAC rip | changed | changed | same-ish | yes |
| two copies of one recording | different | either | same | no — two tracks |
| delete | gone | — | — | no |

## Decision

**The row id is the identity.** Favorites, playlist entries, queue items and
play events reference `track.id` / `album.id`. The scanner's job is to keep a
row when a file turns out to be the same recording at a new path, instead of
deleting and re-inserting it.

**Relink at insert time.** When `processAudioFile` meets a path with no row, it
first looks for a *relink candidate*: a track in the same library whose stored
path no longer exists on disk while the library path it lives under is still
readable (`os.Stat`, checked only for candidates — an unmounted share must not
look like a move). It
tries, in order, and takes the first rule that yields exactly one candidate:

1. same MusicBrainz recording id (`musicbrainz_trackid`)
2. same content hash (see below)
3. same normalised album artist, album title, disc number, track number and
   title, with duration within 2 s

On a match the existing row gets the new path and fresh metadata; its id is
kept. Zero or several candidates means a new track. A file's old path no
longer has a row by the time `removeVanishedTracks` runs, so the end-of-scan
delete needs no change and only removes files that are really gone.

**Content hash is sampled.** A new `track.content_hash` column holds a hash of
the file size plus the first, middle and last 64 KiB. It costs ~192 KiB of
reading per file, not the whole file, and only on new or changed files. It
exists for the plain move/rename case. It changes on retag, which rules 1 and
3 cover. Existing rows get it on the next scan that touches them; until then
they can't match on rule 2.

**Albums follow their tracks.** `GetOrCreateAlbum` matches, in order: the
MusicBrainz release id (which the scanner extracts today but doesn't save),
then the existing `(artist, title, folder_path)`, then the same artist and
title whose `folder_path` no longer exists on disk. That last match updates
`folder_path` and keeps the id. At the end of a scan, albums with no tracks
left are deleted.

**Matching never crosses libraries.** A file moved from one library to another
is a new track.

**Deletes stay hard.** A track or album that is really gone takes its
favorites, playlist entries and queue items with it (`ON DELETE CASCADE`).
Whether play history keeps a trace of deleted tracks is left to the play-log
change.

## Consequences

### Positive

- Renames, folder moves and Picard runs keep favorites, playlists, queue and
  history intact.
- Upgrading a rip to a better format at a new path keeps its favorites and
  play count, through the MBID or tag rule.
- Empty albums stop piling up after folders move.
- The album MusicBrainz id gets persisted, which radio needs anyway.

### Negative

- The scanner gets more logic in its hottest path, so it needs table tests per
  row of the table above.
- A file overwritten in place by a different song keeps the old id. Rare,
  accepted.
- Rule 3 can relink two different recordings that share every tag field,
  e.g. a live version tagged like the studio one. Requiring exactly one
  candidate and a close duration keeps this rare. A wrong relink only carries
  a favorite or play count over to a near-identical track.
- One more column and one more partial file read per new or changed file.
- A move split across two scans (file gone in one, back in the next) is still
  a delete plus a new track.

## Alternatives considered

- **Path as identity (status quo).** Simplest, but breaks on the most common
  library maintenance there is. Rejected.
- **Full-file hash as identity.** Changes on every retag, and the first scan of
  a large FLAC library would read every byte. Rejected as the identity; the
  sampled hash is kept as one matching signal.
- **Acoustic fingerprint (Chromaprint/AcoustID).** Survives any re-encode, but
  it needs `fpcalc` and decoding every file. Too heavy for identity. It may
  come back later for discovery.
- **MusicBrainz id only.** Exact when present, but coverage depends entirely
  on how the user tagged. Kept as the first rule, not the only one.
- **Writing our own UUID into each file's tags.** Most robust, but it modifies
  user files and the library is read-only to us. Rejected.
- **Soft delete with a grace period** (tombstone vanished tracks, relink when
  they reappear in a later scan). It would also cover a move that is split
  across two scans, but it adds a state every query has to filter. Deferred
  until it's actually needed.
