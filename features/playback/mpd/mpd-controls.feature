@desktop @tablet @mobile
Feature: MPD Playback Controls
  As a user
  I want to control MPD playback from my browser
  So that I can manage my music without touching the speaker

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available
    And Music is playing on "Mock MPD E2E"

  Scenario: User pauses MPD playback
    When User pauses playback
    Then "Mock MPD E2E" is paused
    And Player shows paused state

  Scenario: User resumes MPD playback
    Given "Mock MPD E2E" is paused
    When User resumes playback
    Then "Mock MPD E2E" is playing

  Scenario: User skips track on MPD
    Given Current track is "Test Track 1"
    When User skips to next track
    Then "Mock MPD E2E" plays "Test Track 2"

  Scenario: User adjusts MPD volume
    When User sets volume to 50%
    Then "Mock MPD E2E" volume is 50%

  Scenario: User seeks on MPD
    When User seeks to 50% of track
    Then "Mock MPD E2E" playback position is approximately 50%

  Scenario: MPD playback state syncs to UI
    Given Another client changes "Mock MPD E2E" state
    Then Player UI updates to reflect current state
    And Progress bar shows actual position
