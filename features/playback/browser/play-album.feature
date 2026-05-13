@desktop @tablet @mobile
Feature: Play Album
  As a user
  I want to play an album from my library
  So that I can listen to my music

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: User plays album from library
    Given User is on library browse page
    When User plays album "Complete Test Album"
    Then Music is playing
    And Player footer is visible
    And Player shows "Complete Test Album"
