@desktop @tablet @mobile
Feature: Player UI
  As a user
  I want to see playback information
  So that I know what is playing and can control it

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: Player footer appears when music starts
    Given No music is playing
    And Player footer is hidden
    When User plays album "Complete Test Album"
    Then Player footer is visible

  Scenario: Player footer shows track info
    Given User is playing album "Complete Test Album"
    Then Player footer shows track title
    And Player footer shows artist name
    And Player footer shows album art

  Scenario: Player footer shows progress
    Given User is playing album "Complete Test Album"
    Then Player footer shows progress bar
    And Player footer shows current time
    And Player footer shows total duration

  Scenario: Player footer persists across navigation
    Given User is playing album "Complete Test Album"
    When User navigates to settings
    Then Player footer is visible
    And Music is playing

  Scenario: Player can be expanded to full view
    Given User is playing album "Complete Test Album"
    When User expands player
    Then Full player view is visible
    And Full player shows album art large
    And Full player shows track list

  Scenario: Player can be minimized from full view
    Given Full player view is visible
    When User minimizes player
    Then Player footer is visible
    And Full player view is hidden
