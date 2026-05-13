@desktop @tablet @mobile @wip
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
    And User is playing album "Test Album"

  Scenario: User views queue
    Given User has tracks in queue
    When User opens queue view
    Then Queue view shows explicit queue section
    And Queue view shows source section
    And Queue view shows playing from "Test Album"

  Scenario: User removes track from queue
    Given Queue contains "Track A" and "Track B"
    When User removes "Track A" from queue
    Then Queue does not contain "Track A"
    And Queue contains "Track B"

  Scenario: User clears queue
    Given Queue contains multiple tracks
    When User clears queue
    Then Queue is empty
    And Source remains album "Test Album"
    And Music continues playing

  Scenario: User reorders queue
    Given Queue contains "Track A", "Track B", "Track C"
    When User moves "Track C" to position 1
    Then Queue order is "Track C", "Track A", "Track B"

  Scenario: Queue shows track origin
    Given Queue contains "Track A" from album "Album X"
    When User opens queue view
    Then Queue shows "Track A" with source "Album X"

  Scenario: User can play track from queue immediately
    Given Queue contains "Track A", "Track B"
    When User plays "Track B" from queue
    Then Current track is "Track B"
    And "Track A" is removed from queue
