@desktop @tablet @mobile
Feature: Library Editing
  As an admin user
  I want to edit existing libraries
  So that I can update library configuration

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in
    And Library "Test Music" exists with paths "e2e/data/music"
    And Library "Test Music" scan is complete

  Scenario: Admin edits library name
    When User renames library "Test Music" to "Updated Music"
    Then Library "Updated Music" exists
    And Library "Test Music" does not exist

  Scenario: Admin adds path to library
    When User edits library "Test Music" adding path "e2e/data/music/02-hi-res"
    Then Library "Test Music" has paths "e2e/data/music, e2e/data/music/02-hi-res"

  Scenario: Admin removes path from library
    Given Library "Multi Path" exists with paths "e2e/data/music/01-standard, e2e/data/music/02-hi-res"
    And Library "Multi Path" scan is complete
    When User edits library "Multi Path" removing path "e2e/data/music/02-hi-res"
    Then Library "Multi Path" has paths "e2e/data/music/01-standard"

  Scenario: Admin cannot save library with empty name
    When User renames library "Test Music" to ""
    Then Saving the library is not possible

  Scenario: Non-admin without access cannot see library
    Given User "bob" exists with password "bobpass123"
    And User "bob" is logged in
    Then User cannot see library "Test Music"

  Scenario: Non-admin with access cannot edit library
    Given User "bob" exists with password "bobpass123"
    And User "bob" has read access to library "Test Music"
    And User "bob" is logged in
    Then User can see library "Test Music"
    But User cannot edit library "Test Music"
