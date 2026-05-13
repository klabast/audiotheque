@mobile
Feature: Scroll restoration on back-navigation
  # Tagged @mobile only — the scroll mechanism is viewport-independent,
  # but the test fixture (~7 albums) overflows <main> only on mobile.
  As a user browsing my music library
  I want my scroll position preserved when I return from an album
  So that I don't lose my place in a long list

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: Scroll position is preserved when returning from album details
    Given User is on library browse page
    And User has scrolled down in the album grid
    When User opens the last album in the grid
    And User navigates back to library
    Then Album grid is scrolled to the previous position
