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

Prove every change against the real stack before it can merge, and
serialise trunk.

- `pull_request` runs the whole pipeline except promote. Nothing merges
  without having passed acceptance, so trunk cannot go red from a change
  that fails E2E.
- `merge_group` runs the whole pipeline too. GitHub offers merge queues
  for org-owned repositories, and for private repositories on Team and
  above; this repository is owned by a user account, so the trigger never
  fires today. It is wired up anyway so that enabling a queue later — or
  moving the repository to an organisation — is a settings change rather
  than a workflow change.
- `push` to `main` runs the whole pipeline and promotes. Squash-merging
  produces a new sha, so the artifact that is actually released is rebuilt
  and re-verified rather than assumed from the pull request run.

Testing the *combination* of trunk and the change is the one thing a
merge queue would add. Without one, branch protection requires the pull
request branch to be up to date with `main` before it may merge, which
forces the same question to be answered — by updating the branch instead
of by a queue building a speculative merge. It is the same guarantee paid
for with a rebase.

Trunk pipelines are serialised with a `concurrency` group. Superseded pull
request runs are cancelled; trunk and queue runs always finish, because
every commit needs its own verdict.

## Consequences

Merging several pull requests at once stops needing hand-sequencing and a
local trial-merge of each onto trunk. Whoever merges second has to update
their branch first, and that update is what gets tested.

Acceptance runs twice per change — once on the pull request, once on
trunk. That is deliberate: with squash merges the two shas differ, and
releasing an artifact built from a sha that was never tested would give up
the single-artifact property this pipeline exists to protect.

Every push to a pull request now costs an image build plus a three-device
acceptance matrix, roughly ten minutes. Superseded pull request runs are
cancelled to bound the waste; trunk runs never are. This is the trade
trunk-based development is built on: slower to merge, trunk always
releasable.

Required checks are the commit stage plus the three acceptance jobs, with
the branch required to be up to date. The repository admin role keeps a
bypass, so a stuck check can never lock the owner out of their own
repository. Path-filtered workflows (`test-mpd-image`) are deliberately
not required — a required check that does not run on every change blocks
merging entirely.
