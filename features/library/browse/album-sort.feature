@desktop @tablet @mobile
Feature: Two-level album sort
  As a user with a library
  I want to sort albums by two levels (e.g. album artist, then year)
  So I can browse my collection in a meaningful order

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: Default sort is album artist then year, ascending
    Given User is on library browse page
    Then Sort primary is "album-artist" ascending
    And Sort secondary is "year" ascending

  Scenario: Changing primary sort updates the URL
    Given User is on library browse page
    When User sets primary sort to "album-title"
    Then URL contains sort "album-title:asc,year:asc"

  Scenario: Toggling sort direction updates the URL
    Given User is on library browse page
    When User toggles primary sort direction
    Then URL contains sort "album-artist:desc,year:asc"

  Scenario: Sort persists across page refresh
    Given User is on library browse page
    And User sets primary sort to "year"
    When User refreshes the page
    Then Sort primary is "year" ascending
