@desktop @tablet @mobile
Feature: User Logout
  As a logged-in Audiotheque user
  I want to log out of my account
  So that others cannot access my music library

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"

  Scenario: User logs out successfully
    Given User "alice" is logged in
    When User logs out
    Then User should be logged out
    And User is on login page
