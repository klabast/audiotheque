@desktop @tablet @mobile
Feature: Queue Management
  As a user
  I want to manage my playback queue
  So that I can control what plays next

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And User is playing album "Complete Test Album"

  Scenario: Queue shows where a track came from
    Given Queue contains "Long Track 1"
    When User opens queue sidebar
    Then Queue shows "Long Track 1" from album "Test Album - Multi Long"

  Scenario: User removes a track from the queue
    Given Queue contains "Long Track 1", "Long Track 2"
    When User removes "Long Track 1" from queue
    Then Queue order is "Long Track 2"

  Scenario: User clears the queue
    Given Queue contains "Long Track 1", "Long Track 2"
    When User clears queue
    Then Queue is empty
    And Source remains album "Complete Test Album"
    And Music is playing

  Scenario: User moves a track to the front
    Given Queue contains "Long Track 1", "Long Track 2", "Hi-Res Test Track"
    When User moves "Hi-Res Test Track" to position 1
    Then Queue order is "Hi-Res Test Track", "Long Track 1", "Long Track 2"

  Scenario: User jumps to a queued track
    Given Queue contains "Long Track 1", "Long Track 2"
    When User plays "Long Track 2" from queue
    Then Now playing is "Long Track 2"
    And Queue is empty
