@desktop @tablet @mobile
Feature: Play Next
  As a user
  I want to put tracks right after the current one
  So that I hear them next without losing the rest of my queue

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And User is playing album "Complete Test Album"

  Scenario: Play-next track goes to the front of the queue
    Given Queue contains "Long Track 1"
    When User adds track "Long Track 2" to play next
    Then Queue order is "Long Track 2", "Long Track 1"
    And Source remains album "Complete Test Album"

  Scenario: User plays a whole album next
    Given Queue contains "Hi-Res Test Track"
    When User adds album "Test Album - Multi Long" to play next
    Then Queue order is "Long Track 1", "Long Track 2", "Hi-Res Test Track"
