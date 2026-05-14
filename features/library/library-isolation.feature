@desktop @tablet @mobile
Feature: Per-User Library Isolation
  As an Audiotheque admin sharing a server with other admins
  I want each user's libraries to be invisible to the others by default
  So that personal collections stay private without an extra access step

  # The library_access table already enforces this server-side: every
  # ListLibrariesForUser INNER JOINs through it, so admin-ness alone does
  # NOT grant cross-user library visibility. These scenarios pin that down
  # at the UI level — they would have caught a regression where, e.g.,
  # an admin "sees all libraries" shortcut got added to the handler.

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Admin-User "bob" exists with password "bobpass456"

  Scenario: Two admins each see only their own library
    Given User "alice" is logged in
    And User creates library named "Alice's Music" with path "e2e/data/music"
    When User logs out
    And User "bob" is logged in
    Then User cannot see library "Alice's Music"
    When User creates library named "Bob's Music" with path "e2e/data/music"
    Then User can see library "Bob's Music"
    When User logs out
    And User "alice" is logged in
    Then User can see library "Alice's Music"
    And User cannot see library "Bob's Music"
