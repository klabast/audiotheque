@desktop @tablet @mobile
Feature: Session Persistence
  As an Audiotheque user
  I want my session to stay valid as long as I keep using the app
  So that I don't have to log in every visit

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"

  Scenario: Default session lasts about 30 days
    Given User is on login page
    When User authenticates with username "alice" and password "alicepass123" without keeping logged in
    Then User should be logged in as "alice"
    And Session is set to expire in approximately 30 days

  Scenario: "Keep me logged in" extends the session window to about 90 days
    Given User is on login page
    When User authenticates with username "alice" and password "alicepass123" and keeps logged in
    Then User should be logged in as "alice"
    And Session is set to expire in approximately 90 days

  Scenario: Continued use renews the session window
    Given User "alice" is logged in
    And Session is past the halfway point of its window
    When User browses the library
    Then Session expiry is renewed

  Scenario: An invalid or stale session cookie is rejected
    Given User has a stale session cookie for "alice"
    When User navigates to the application
    Then User should see the login page
