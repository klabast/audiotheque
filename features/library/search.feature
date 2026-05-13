@desktop @tablet @mobile
Feature: Library search
  As a user with many albums and artists
  I want to find tracks, albums, and artists by name
  So I can play music quickly

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: Search returns matching albums
    Given User is on library browse page
    When User searches for "album"
    Then Search results include at least one album

  Scenario: Search shows empty state for no matches
    Given User is on library browse page
    When User searches for "qzxqzxqzx-not-found"
    Then Search results are empty

  Scenario: Cmd+K focuses the search input
    Given User is on library browse page
    When User presses the search shortcut
    Then Search input is focused

  Scenario: Slash key focuses the search input
    Given User is on library browse page
    When User presses the slash key
    Then Search input is focused
