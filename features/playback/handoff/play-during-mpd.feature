@desktop @tablet @mobile
Feature: Play another album while MPD is active
  As a user listening through MPD
  When I pick a different album to play
  I want it to keep playing on MPD
  So that my speaker selection isn't silently overridden by the browser

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available

  Scenario: Picking a new album keeps playback on MPD
    Given Music is playing on "Mock MPD E2E"
    When User plays album "Complete Test Album"
    Then "Mock MPD E2E" is playing
    And Browser audio stops
    And Player shows playing on "Mock MPD E2E"
