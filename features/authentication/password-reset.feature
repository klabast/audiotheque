@desktop @tablet @mobile
Feature: Password Reset
  As an Audiotheque user who forgot their password
  I want to reset my password using a recovery code
  So that I can regain access to my account

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"

  Scenario: Logged-out user resets password
    Given User is logged out
    When User requests password reset for "alice"
    Then Reset code is generated for "alice"
    When User enters valid reset code
    And User resets password to "newpassalice123"
    Then User is on login page
    And User authenticates with username "alice" and password "newpassalice123"
    And User is on library page
