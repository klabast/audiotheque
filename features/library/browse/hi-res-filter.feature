@desktop @tablet @mobile
Feature: Hi-res album filter
  As a user with both hi-res and standard albums
  I want to filter the library to show only hi-res content
  So I can browse my best-quality recordings

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: Hi-res toggle filters out standard-quality albums
    Given User is on library browse page
    And Library shows both hi-res and standard albums
    When User enables the hi-res filter
    Then Only hi-res albums are visible

  Scenario: Disabling hi-res filter restores all albums
    Given User is on library browse page
    And Library shows both hi-res and standard albums
    And User enables the hi-res filter
    When User disables the hi-res filter
    Then All albums are visible

  Scenario: Hi-res filter persists across page refresh
    Given User is on library browse page
    And User enables the hi-res filter
    When User refreshes the page
    Then Only hi-res albums are visible
    And Hi-res filter is enabled
