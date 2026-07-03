@desktop @tablet @mobile
Feature: Change Password
  As a logged-in Audiotheque user
  I want to change my password
  So that I can maintain account security

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in

  Scenario: User changes password successfully
    When User changes password from "alicepass123" to "newpass456"
    Then Password change succeeds
    When User logs out
    And User authenticates with username "alice" and password "newpass456"
    Then User should be logged in as "alice"

  Scenario: User cannot change password with mismatched confirmation
    When User attempts to change password with mismatched confirmation
    Then User should see error "Passwords do not match"

  Scenario: User cannot change password with wrong current password
    When User attempts to change password with wrong current password
    Then User should see error "Current password is incorrect"

  Scenario: A very short password is warned about but still accepted
    When User starts changing password from "alicepass123" to "p"
    Then Weak password warning is shown
    When User submits the password change
    Then Password change succeeds
    When User logs out
    And User authenticates with username "alice" and password "p"
    Then User should be logged in as "alice"

  Scenario: A password matching the username is warned about but still accepted
    When User starts changing password from "alicepass123" to "alice"
    Then Weak password warning is shown
    When User submits the password change
    Then Password change succeeds
