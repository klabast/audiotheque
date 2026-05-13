@desktop @tablet @mobile
Feature: Album Details
  As a user browsing my music library
  I want to view album details when I click on an album
  So that I can see the tracks and additional information

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: User navigates to album details
    Given User is on library browse page
    When User clicks on an album
    Then User sees album details page

  Scenario: Album details shows tracks
    Given User is on library browse page
    When User clicks on an album
    Then User sees track list for the album

  Scenario: User can navigate back to library
    Given User is on album details page
    When User navigates back to library
    Then User is on library browse page
