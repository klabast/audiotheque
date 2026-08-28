# 1. Trunk-based delivery with a merge queue

Date: 2026-08-28

## Status

Accepted

## Context

The pipeline was already Farley-shaped in the part that is hardest to get
right: the image is built once per commit and promoted to `:latest` by
retag, never rebuilt, so what ships is bit-identical to what passed
acceptance. The commit stage is fast (~1–2 min per side).

Three things were missing, and all three showed up in a single afternoon
of merging three PRs:

**Acceptance only ran after merge.** `image`, `acceptance` and `promote`
were gated on `push` to `main`, so a pull request never produced a release
candidate and never ran E2E. A rate-limit change that was green on its PR
would have broken trunk, because the E2E suite runs three device passes
against one server from one address and accumulated more credential
failures than the new per-IP limit allowed. It was caught by reading the
feature files, not by the pipeline. The old comment in `ci.yml` named this
trade explicitly — "we accept that a broken main needs a fast revert" —
but the cost lands on trunk, which is the one place it must not.

**Nothing tested the combination.** Each PR was verified against `main` as
it was when the branch was cut, never against `main` as it would be. Two
independently green PRs can break together.

**Promote was an unguarded race.** With no `concurrency` group, two merges
minutes apart produce two overlapping trunk pipelines, both ending in
`docker buildx imagetools create --tag :latest`. Last writer wins with no
ordering guarantee, so `:latest` can land on the older commit while every
job is green — a silent regression with no failure signal. Two such runs
did overlap in practice; the order happened to hold, but nothing enforced
it.

## Decision

Run the full pipeline on `merge_group` and make the merge queue the only
route to trunk.

- `pull_request` runs the commit stage only. The PR head is not what
  lands, so a candidate built from it is a candidate for something that
  never ships. Fast signal while iterating.
- `merge_group` runs the whole pipeline — image and acceptance included —
  against the prospective combination of trunk plus the queued change.
  This is the gate.
- `push` to `main` runs the whole pipeline and promotes. Squash-merging
  produces a new sha, so the artifact that is actually released is rebuilt
  and re-verified rather than assumed from the queue.

Trunk pipelines are serialised with a `concurrency` group. Superseded pull
request runs are cancelled; trunk and queue runs always finish, because
every commit needs its own verdict.

## Consequences

Merging several pull requests at once becomes safe by construction rather
than by hand-sequencing them and locally trial-merging each onto trunk.

Acceptance runs twice per change — once in the queue, once on trunk. That
is deliberate: with squash merges the queue sha and the trunk sha differ,
and releasing an artifact built from a sha that was never tested would
give up the single-artifact property this pipeline exists to protect.

Merging is slower per change: a queued PR waits for image plus a
three-device acceptance matrix. In exchange trunk stays green, which is
the trade trunk-based development is built on.

Required checks are the commit stage plus the three acceptance jobs. The
repository admin role can bypass, so a stuck queue can never lock the
owner out of their own repository. Path-filtered workflows
(`test-mpd-image`) are deliberately not required, since a required check
that does not run on every merge group deadlocks the queue.
