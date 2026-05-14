@desktop @tablet @mobile
Feature: Sudo Confirmation for Sensitive Operations
  As an Audiotheque user
  I want sensitive operations to require my password again
  So that an unattended logged-in session can't be hijacked into doing damage

  # The canonical sensitive op exercised here is "Log out of all devices" on
  # the Security tab — it's the most destructive currently shipped action
  # (wipes every session for the user, including the current one). The same
  # modal will gate "Disable authentication" once that feature lands.

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in

  Scenario: Sensitive operation prompts for password re-confirmation
    When User attempts to log out of all devices
    Then User is prompted to confirm their password

  Scenario: Correct password completes the sensitive operation
    When User attempts to log out of all devices
    And User confirms with password "alicepass123"
    Then User should be logged out

  Scenario: Wrong password is rejected and the operation does not proceed
    When User attempts to log out of all devices
    And User confirms with password "wrongpassword"
    Then Sudo confirmation is rejected
    And User remains logged in as "alice"
