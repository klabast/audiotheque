@desktop @tablet @mobile @wip
Feature: Source Playlist
  As a user
  I want to play albums and playlists as my source
  So that I have continuous playback from a collection

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: Playing album sets source
    When User plays album "Test Album"
    Then Source is album "Test Album"
    And Source shows remaining tracks

  Scenario: Source tracks play after queue empties
    Given User is playing album "Test Album" from track 1
    And Queue is empty
    When Current track finishes
    Then Current track is next track from source
    And Source remaining count decreases

  Scenario: Changing source warns about replacement
    Given User is playing album "Test Album"
    And Queue contains tracks
    When User starts playing album "Another Album"
    Then User is warned about replacing current session
    And User can confirm or cancel

  Scenario: Confirming source change replaces session
    Given User is playing album "Test Album"
    When User confirms playing album "Another Album"
    Then Source is album "Another Album"
    And Queue is empty
    And Previous session is saved to history

  Scenario: Canceling source change keeps current session
    Given User is playing album "Test Album"
    When User cancels playing album "Another Album"
    Then Source remains album "Test Album"
    And Queue is unchanged

  Scenario: Session history allows restore
    Given User has previous session with album "Old Album"
    When User restores previous session
    Then Source is album "Old Album"
    And Queue is restored from previous session

  Scenario: Shuffle mode randomizes source order
    Given User is playing album "Test Album"
    When User enables shuffle
    Then Source tracks are in random order
    And All tracks will eventually play

  Scenario: Repeat mode loops source
    Given User is playing album "Test Album"
    And Repeat is enabled
    When Last track of source finishes
    Then Playback continues from first track
    And Source is reset to full album
