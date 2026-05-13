@desktop @tablet @mobile
Feature: Library Visibility
  As a user with library access
  I want to see albums in my library
  So that I can browse my music collection

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: User sees albums from their library
    When User navigates to library browse
    Then User sees albums in the library

  Scenario: Albums display cover art
    When User navigates to library browse
    Then User sees albums with cover images

  Scenario: Albums display title and artist
    When User navigates to library browse
    Then User sees album titles
    And User sees album artists
