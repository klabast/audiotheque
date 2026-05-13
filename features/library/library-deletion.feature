@desktop @tablet @mobile
Feature: Library Deletion
  As an admin user
  I want to delete libraries
  So that I can remove unused library configurations

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in
    And Library "Test Music" exists with path "e2e/data/music"
    And Library "Test Music" scan is complete

  Scenario: Admin deletes library successfully
    When User deletes library "Test Music"
    Then Library "Test Music" does not exist
    And Library list does not contain "Test Music"

  Scenario: Non-admin with access cannot delete library
    Given User "bob" exists with password "bobpass123"
    And User "bob" has read access to library "Test Music"
    And User "bob" is logged in
    Then User can see library "Test Music"
    But User cannot delete library "Test Music"
