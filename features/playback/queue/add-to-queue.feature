@desktop @tablet @mobile @wip
Feature: Add to Queue
  As a user
  I want to add tracks to the end of my queue
  So that I can build a playlist of songs to hear later

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And User is playing album "Test Album"

  Scenario: User adds track to queue
    Given Current track is "Test Track 1"
    When User adds "Another Track" to queue
    Then Queue contains "Another Track" at last position
    And Source remains album "Test Album"

  Scenario: Queue tracks play after explicit queue empties
    Given Current track is "Test Track 1"
    And User has added "Queue Track" to queue
    When "Queue Track" finishes playing
    Then Current track is "Test Track 2" from source

  Scenario: Multiple add to queue tracks append
    Given Current track is "Test Track 1"
    When User adds "Track A" to queue
    And User adds "Track B" to queue
    Then Queue order is "Track A", "Track B"

  Scenario: Add to queue from album details
    Given User is on album details page for "Complete Test Album"
    When User adds track "Song X" to queue
    Then Queue contains "Song X"
    And User remains on album details page

  Scenario: Add entire album to queue
    Given Current track is "Test Track 1"
    When User adds album "Another Album" to queue
    Then Queue contains all tracks from "Another Album" at end
