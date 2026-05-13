@desktop @tablet @mobile
Feature: MPD device failure surfaces an error
  As a user trying to play to an MPD speaker
  When the speaker is unreachable
  I want the failure surfaced
  So that playback doesn't silently move into an invalid state

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: Transfer to an unreachable MPD device fails and keeps playback in browser
    Given An unreachable MPD device "Broken MPD" is registered
    And User is playing album "Complete Test Album" in browser
    When User attempts to transfer playback to "Broken MPD"
    Then Transfer request fails
    And Session remains in browser
