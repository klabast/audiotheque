@desktop @tablet @mobile
Feature: MPD to Browser Handoff
  As a user
  I want to transfer playback from MPD to my browser
  So that I can continue listening on my device

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available
    And Music is playing on "Mock MPD E2E"

  Scenario: User transfers playback to browser
    Given Current track is "Test Track 2" at 0:32
    When User selects "Play Here" on browser
    Then Browser plays "Test Track 2"
    And Playback continues from approximately 0:32

  Scenario: MPD stops after handoff to browser
    When User transfers playback to browser
    Then "Mock MPD E2E" stops playback
    And Browser is now playing

  Scenario: Handoff preserves session state
    Given Queue contains tracks
    And Source is album "Test Album"
    When User transfers playback to browser
    Then Queue is preserved
    And Source is preserved

  @wip
  Scenario: Multiple browsers show play here option
    Given User has browser on laptop and phone
    When User opens device picker on phone
    Then Device picker shows "This Device" option
    And Device picker shows "Mock MPD E2E" as current

  @wip
  Scenario: Taking over from MPD notifies other clients
    Given Another user's browser is showing MPD playback
    When User transfers playback to their browser
    Then Other browsers update to show new playback location
