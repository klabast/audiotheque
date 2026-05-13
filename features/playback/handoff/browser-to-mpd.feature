@desktop @tablet @mobile
Feature: Browser to MPD Handoff
  As a user
  I want to transfer playback from my browser to MPD
  So that I can switch to my speakers seamlessly

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available
    And User is playing album "Test Album - Multi Long" in browser

  Scenario: Seamless handoff preserves playback position
    Given Current track is "Test Track 2" at 1:30
    When User transfers playback to "Mock MPD E2E"
    Then "Mock MPD E2E" plays "Test Track 2"
    And Playback continues from approximately 1:30
    And No audible gap in playback

  @wip
  Scenario: Handoff preserves queue
    Given Queue contains "Extra Track 1", "Extra Track 2"
    When User transfers playback to "Mock MPD E2E"
    Then Queue still contains "Extra Track 1", "Extra Track 2"

  Scenario: Handoff preserves source
    Given Source is album "Test Album - Multi Long" with tracks remaining
    When User transfers playback to "Mock MPD E2E"
    Then Source is still album "Test Album - Multi Long"
    And Same tracks remain in source

  Scenario: Browser stops playback after handoff
    When User transfers playback to "Mock MPD E2E"
    Then Browser audio stops
    And Browser shows "Mock MPD E2E"

  Scenario: Handoff during track transition
    Given Current track is about to end
    When User transfers playback to "Mock MPD E2E"
    Then Handoff completes cleanly
    And Next track plays on "Mock MPD E2E"

  Scenario: Failed handoff recovers gracefully
    Given "Mock MPD E2E" becomes unavailable during handoff
    When Handoff fails
    Then Browser resumes playback
    And User is notified of handoff failure
