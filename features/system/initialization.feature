@desktop @tablet @mobile
Feature: System Initialization
  As a new Audiotheque user
  I want to create my admin account on first launch
  So that I can secure my music server with my own credentials

  Background:
    Given System is in initial state

  Scenario: System requiring initialization shows init page
    When User navigates to the application
    Then User should see the initialization page
    And User should see a form to create the first account

  Scenario: Create first admin account
    Given User is on the initialization page
    When User creates account with username "alice" and password "alicepass123"
    Then User should be logged in as "alice"
    And User should see the library page

  Scenario: Initialization page not accessible after initialization is complete
    Given Admin-User "admin" exists with password "admin"
    When User attempts to access the initialization page
    Then User should see the login page

  Scenario: Application redirects to init when initialization required
    When User visits any page
    Then User should see the initialization page

  @wip
  Scenario: First admin creates an account and disables login at setup
    Given User is on the initialization page
    When User creates account with username "alice" and password "alicepass123" and disables login
    Then User should be logged in as "alice"
    And Authentication is disabled
