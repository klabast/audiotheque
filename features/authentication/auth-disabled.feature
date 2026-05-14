@desktop @tablet @mobile
Feature: Disabling Login
  As an Audiotheque owner running on a trusted home network
  I want the option to turn login off entirely
  So that I don't have to authenticate every time I open the app

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in

  Scenario: With login disabled, every visit auto-logs in as the admin
    Given Authentication is disabled
    When User navigates to the application
    Then User should be logged in as "alice"

  Scenario: With login disabled, user management is unavailable
    Given Authentication is disabled
    When User opens user management in settings
    Then User management is unavailable with an explanation

  Scenario: Disabling login asks for confirmation
    When User attempts to disable authentication
    Then User sees the disable-login warning

  Scenario: Cancelling the disable-login warning keeps login enabled
    When User attempts to disable authentication
    And User cancels the disable-login warning
    Then Authentication is enabled

  Scenario: Confirming the disable-login warning turns login off
    When User attempts to disable authentication
    And User confirms the disable-login warning with password "alicepass123"
    Then Authentication is disabled

  Scenario: Admin re-enables login from settings
    Given Authentication is disabled
    When User re-enables authentication with password "alicepass123"
    Then User should see the login page
