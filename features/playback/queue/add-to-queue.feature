@desktop @tablet @mobile
Feature: Add to Queue
  As a user
  I want to add tracks to the end of my queue
  So that I hear them after what I already queued, before the album carries on

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And User is playing album "Complete Test Album"

  Scenario: Added track goes to the end of the queue
    Given Queue contains "Long Track 1"
    When User adds track "Long Track 2" to queue
    Then Queue order is "Long Track 1", "Long Track 2"
    And Source remains album "Complete Test Album"

  Scenario: Queued track plays before the album carries on
    Given Queue contains "Long Track 1"
    When Current track finishes
    Then Now playing is "Long Track 1"
    And Next from source is "Track 02 - Album Test"

  Scenario: User adds a whole album to the queue
    Given Queue contains "Hi-Res Test Track"
    When User adds album "Test Album - Multi Long" to queue
    Then Queue order is "Hi-Res Test Track", "Long Track 1", "Long Track 2"
