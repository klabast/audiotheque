@desktop @tablet @mobile
Feature: Play to MPD Device
  As a user
  I want to play music on my MPD speakers
  So that I can listen on my home audio system

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available

  Scenario: User plays album to MPD device
    Given User is on library browse page
    When User plays album "Complete Test Album" to device "Mock MPD E2E"
    Then Music is playing on "Mock MPD E2E"
    And Player shows playing on "Mock MPD E2E"

  Scenario: User selects MPD device from device picker
    Given User is playing album "Complete Test Album" in browser
    When User opens device picker
    And User selects device "Mock MPD E2E"
    Then Music is playing on "Mock MPD E2E"
    And Browser playback stops

  Scenario: MPD device shows in device list
    When User opens device picker
    Then Device list shows "Mock MPD E2E"
    And Device list shows "This Device"

  Scenario: Playing to MPD transfers current session
    Given User is playing album "Test Album - Multi Long" in browser
    And Current track is "Test Track 2" at 1:30
    When User transfers playback to "Mock MPD E2E"
    Then "Mock MPD E2E" plays "Test Track 2" from approximately 1:30
    And Queue is preserved
    And Source is preserved

  Scenario: User can control MPD from any client
    Given Music is playing on "Mock MPD E2E"
    When User pauses playback
    Then "Mock MPD E2E" pauses playback
    And All connected clients show paused state
