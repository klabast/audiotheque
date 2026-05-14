@desktop @tablet @mobile
Feature: Weak Password Warning
  As an Audiotheque user setting a password
  I want a non-blocking hint when my password is too short
  So that I'm encouraged to pick something stronger without being forced into pointless character-class rules

  Background:
    Given System is in initial state

  Scenario: A short password shows a non-blocking warning on first-run setup
    Given User is on the initialization page
    When User enters a weak password
    Then Weak password warning is shown

  Scenario: A long password does not trigger the warning
    Given User is on the initialization page
    When User enters a strong password
    Then Weak password warning is not shown

  Scenario: A password matching the chosen username triggers a warning
    Given User is on the initialization page
    When User enters username "alice" and a password matching the username
    Then Weak password warning is shown

  Scenario: The warning does not block account creation
    Given User is on the initialization page
    When User creates account with username "alice" and a weak password
    Then User should be logged in as "alice"
