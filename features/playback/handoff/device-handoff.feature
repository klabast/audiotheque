@desktop
Feature: Device Handoff - Thorough
  As a user switching between speakers and headphones
  I want playback to transfer seamlessly between devices
  So that I never lose my place or have to readjust volume

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available

  Scenario: Browser to MPD preserves position
    Given Music is playing on browser
    And Playback position is at 45 seconds
    When User transfers playback to "Mock MPD E2E"
    Then "Mock MPD E2E" is playing
    And "Mock MPD E2E" playback position is approximately 45 seconds

  Scenario: MPD to browser preserves position
    Given Music is playing on "Mock MPD E2E"
    And "Mock MPD E2E" playback position is at 60 seconds
    When User transfers playback to browser
    Then Session position is approximately 60 seconds

  Scenario: Round-trip Browser to MPD to Browser preserves position
    Given Music is playing on browser
    And Playback position is at 30 seconds
    When User transfers playback to "Mock MPD E2E"
    Then "Mock MPD E2E" playback position is approximately 30 seconds
    When User seeks to 75 seconds on "Mock MPD E2E"
    And User transfers playback to browser
    Then Session position is approximately 75 seconds

  Scenario: Per-device volume is remembered across transfers
    Given Music is playing on browser
    When User sets volume to 80% via API
    And User transfers playback to "Mock MPD E2E"
    And User sets volume to 40% via API
    And User transfers playback to browser
    Then Session device volume for browser is 80%
    When User transfers playback to "Mock MPD E2E"
    Then "Mock MPD E2E" volume is 40%

  Scenario: Multiple back-and-forth transfers keep correct state
    Given Music is playing on browser
    And Playback position is at 10 seconds
    When User transfers playback to "Mock MPD E2E"
    And User seeks to 50 seconds on "Mock MPD E2E"
    And User transfers playback to browser
    And User seeks to 80 seconds via API
    And User transfers playback to "Mock MPD E2E"
    Then "Mock MPD E2E" playback position is approximately 80 seconds

  Scenario: Transfer preserves queue and source
    Given Music is playing on browser
    When User transfers playback to "Mock MPD E2E"
    Then Session source is preserved
    When User transfers playback to browser
    Then Session source is preserved

  Scenario: Skip track on MPD then transfer back to browser
    Given Music is playing on "Mock MPD E2E"
    When User skips to next track
    And User transfers playback to browser
    Then Session is on the second track

  Scenario: Pause on MPD then transfer to browser stays paused
    Given Music is playing on "Mock MPD E2E"
    When User pauses playback
    And User transfers playback to browser
    Then Session state is paused
