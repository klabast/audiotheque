@desktop @tablet @mobile
Feature: User Login
  As an Audiotheque user
  I want to log in with my credentials
  So that I can access my music library

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"

  Scenario: User logs in with valid credentials
    Given User is on login page
    When User authenticates with username "alice" and password "alicepass123"
    Then User should be logged in as "alice"
    And User is on library page

  Scenario: User cannot log in with wrong password
    Given User is on login page
    When User authenticates with username "alice" and password "wrongpassword"
    Then User should see error "Invalid username or password"
    And User is on login page

  Scenario: User cannot log in with non-existent username
    Given User is on login page
    When User authenticates with username "nonexistent" and password "anypassword"
    Then User should see error "Invalid username or password"
    And User is on login page
