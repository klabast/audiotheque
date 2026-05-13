@desktop @tablet @mobile
Feature: Library Scanning
  As an admin user
  I want to scan libraries for music files
  So that my music collection is indexed and browsable

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in

  Scenario: New library is automatically scanned after creation
    When User creates library named "Test Music" with path "e2e/data/music"
    Then Library "Test Music" shows track count > 0

  Scenario: Admin manually triggers library scan
    Given Library "Test Music" exists with path "e2e/data/music"
    When User triggers scan for library "Test Music"
    Then Library "Test Music" shows track count > 0

  Scenario: Library track count increases after adding path
    Given Library "Test Music" exists with path "e2e/data/music/01-standard"
    And Library "Test Music" shows track count > 0
    When User edits library "Test Music" adding path "e2e/data/music/02-hi-res"
    Then Library "Test Music" track count has increased

  Scenario: User without library access cannot see library
    Given Library "Test Music" exists with path "e2e/data/music"
    And User "bob" exists with password "bobpass123"
    And User "bob" is logged in
    Then User cannot see library "Test Music"
