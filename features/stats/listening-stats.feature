@desktop @tablet @mobile
Feature: Listening stats
  As a user
  I want to see what I listen to most
  So that I find my way back to the music I love

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And User is playing album "Complete Test Album"

  Scenario: Tracks listened to rank by how often
    Given User has listened to "Long Track 1" 2 times
    And User has listened to "Long Track 2" 3 times
    When User looks at stats for this week
    Then Top track is "Long Track 2" with 3 plays
    And Stats count 5 listens

  Scenario: Skipped tracks don't count as listens
    Given User has skipped "Long Track 1"
    When User looks at stats for this week
    Then Stats count no listens
