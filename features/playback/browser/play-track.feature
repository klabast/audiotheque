@desktop @tablet @mobile
Feature: Play Track
  As a user
  I want to play individual tracks
  So that I can listen to specific songs

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: User plays track from album details
    Given User is on album details page for "Complete Test Album"
    When User plays track "Track 01 - Album Test"
    Then Music is playing
    And Player shows "Track 01 - Album Test"

  Scenario: User plays track and album becomes source
    Given User is on album details page for "Complete Test Album"
    When User plays track "Track 02 - Album Test"
    Then Music is playing
    And Source is album "Complete Test Album"
    And Next track is "Track 03 - Album Test"
