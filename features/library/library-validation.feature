@desktop @tablet @mobile
Feature: Library Validation and Error Handling
  As an admin user
  I want proper validation when managing libraries
  So that I cannot create invalid configurations

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in

  Scenario: Cannot create library with empty name
    When User creates library named "" with path "e2e/data/music"
    Then Library creation is rejected

  Scenario: Cannot create library without paths
    When User creates library named "Test" with path ""
    Then Library creation is rejected

  Scenario: Invalid path handling
    When User creates library named "Test" with path "/nonexistent/path"
    Then Library creation is rejected
