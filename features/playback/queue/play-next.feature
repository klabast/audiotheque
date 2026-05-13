@desktop @tablet @mobile @wip
Feature: Play Next
  As a user
  I want to add tracks to play next
  So that I can hear specific songs after the current one

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And User is playing album "Test Album"

  Scenario: User adds track to play next
    Given Current track is "Test Track 1"
    When User adds "Another Track" to play next
    Then Queue contains "Another Track" at position 1
    And Source remains album "Test Album"

  Scenario: Play next track plays before source
    Given Current track is "Test Track 1"
    And User has added "Another Track" to play next
    When Current track finishes
    Then Current track is "Another Track"
    And Next track is "Test Track 2" from source

  Scenario: Multiple play next tracks stack
    Given Current track is "Test Track 1"
    When User adds "Track A" to play next
    And User adds "Track B" to play next
    Then Queue order is "Track B", "Track A"
    And "Track B" plays after current track

  Scenario: Play next from album details
    Given User is on album details page for "Complete Test Album"
    When User adds track "Song X" to play next
    Then Queue contains "Song X" at position 1
    And User remains on album details page

  Scenario: Play next entire album
    Given Current track is "Test Track 1"
    When User adds album "Another Album" to play next
    Then Queue contains all tracks from "Another Album"
    And Source remains album "Test Album"
