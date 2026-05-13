@desktop @tablet @mobile
Feature: MPD auto-advance to next track
  As a user listening to an album on MPD
  When a track finishes
  I want the next track to play automatically
  So that the album plays through without intervention

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available

  Scenario: Track ending on MPD advances the session to the next track
    Given Music is playing on "Mock MPD E2E"
    When Current track finishes on MPD
    Then Session advances to next track
    And "Mock MPD E2E" is playing

  # Regression coverage for the audio-wz silent-progress bug. Even when MPD
  # plays correctly, the position poller has to mirror MPD's elapsed back
  # to session.current.position and broadcast it over WS, otherwise the
  # seek bar never moves and the user thinks playback stalled.
  Scenario: Playback progress is reported from MPD to the player
    Given Music is playing on "Mock MPD E2E"
    When 4 seconds pass on MPD
    Then Session position advances past 2 seconds
    And Player seek bar shows progress past 2 seconds
