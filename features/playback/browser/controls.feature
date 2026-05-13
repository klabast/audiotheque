@desktop @tablet @mobile
Feature: Playback Controls
  As a user
  I want to control music playback
  So that I can pause, resume, skip, and adjust volume

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And User is playing album "Complete Test Album"

  Scenario: User pauses playback
    Given Music is playing
    When User pauses playback
    Then Music is paused
    And Player shows paused state

  Scenario: User resumes playback
    Given Music is paused
    When User resumes playback
    Then Music is playing

  Scenario: User skips to next track
    Given Music is playing
    When User skips to next track
    Then Music is playing

  Scenario: User goes to previous track
    Given Music is playing
    When User goes to previous track
    Then Music is playing

  Scenario: User seeks within track
    Given Music is playing
    When User seeks to 50% of track
    Then Playback position is approximately 50%

  Scenario: User adjusts volume
    Given Music is playing
    When User sets volume to 50%
    Then Volume is 50%

  Scenario: User mutes playback
    Given Music is playing
    When User mutes playback
    Then Volume is muted

  Scenario: User unmutes playback
    Given Music is playing
    When User mutes playback
    And User unmutes playback
    Then Volume is not muted
