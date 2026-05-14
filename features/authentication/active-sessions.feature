@desktop @tablet @mobile
Feature: Active Device Sessions
  As an Audiotheque user
  I want to see and manage the devices that are signed in to my account
  So that I can revoke access if a device is lost or no longer trusted

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"

  Scenario: User sees their own active sessions in security settings
    Given User "alice" is logged in
    When User opens active devices in security settings
    Then User sees 1 active session
    And Current session is marked as current

  Scenario: User sees all of their active sessions across devices
    Given User "alice" is logged in on browser "A"
    And User "alice" is logged in on browser "B"
    When User on browser "A" opens active devices in security settings
    Then User sees 2 active sessions
    And Current session is marked as current

  Scenario: User revokes a session on another device
    Given User "alice" is logged in on browser "A"
    And User "alice" is logged in on browser "B"
    When User on browser "A" revokes the session on browser "B"
    Then Browser "A" remains logged in as "alice"
    And Browser "B" is logged out on next request

  Scenario: User logs out of all other devices but stays signed in here
    Given User "alice" is logged in on browser "A"
    And User "alice" is logged in on browser "B"
    And User "alice" is logged in on browser "C"
    When User on browser "A" logs out of all other devices
    Then Browser "A" remains logged in as "alice"
    And Browser "B" is logged out on next request
    And Browser "C" is logged out on next request

  Scenario: User logs out of all devices including the current one
    Given User "alice" is logged in on browser "A"
    And User "alice" is logged in on browser "B"
    When User on browser "A" logs out of all devices
    Then Browser "A" is logged out
    And Browser "B" is logged out on next request
