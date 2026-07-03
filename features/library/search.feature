@desktop @tablet @mobile
Feature: Library search
  As a user with many albums and artists
  I want to filter my library live as I type
  So I can find albums, artists, and tracks quickly

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: Typing filters the album grid live, by title
    Given User is on library browse page
    When User searches for "Complete Test"
    Then Album grid shows a matching album
    And Search scope tabs are visible

  Scenario: Typing filters the album grid live, by artist name
    Given User is on library browse page
    When User searches for "Audiotheque Test Band"
    Then Album grid shows a matching album

  Scenario: No matches shows the empty state
    Given User is on library browse page
    When User searches for "qzxqzxqzx-not-found"
    Then Search results are empty

  Scenario: Artists scope filters by artist name only
    Given User is on library browse page
    When User searches for "Complete Test Album"
    And User selects the "artists" search scope
    Then Search results are empty

  Scenario: Tracks scope shows a track result list
    Given User is on library browse page
    When User searches for "Track 01"
    And User selects the "tracks" search scope
    Then Track search results include at least one track

  Scenario: Clearing the search restores the full library view
    Given User is on library browse page
    When User searches for "Complete Test"
    And User clears the search
    Then Search scope tabs are hidden

  Scenario: Typing from another page navigates to the library page
    Given User is on album details page
    When User searches for "Complete Test"
    Then User is redirected to the library browse page
    And Album grid shows a matching album

  Scenario: Cmd+K focuses the search input
    Given User is on library browse page
    When User presses the search shortcut
    Then Search input is focused

  Scenario: Slash key focuses the search input
    Given User is on library browse page
    When User presses the slash key
    Then Search input is focused
