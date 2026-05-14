@wip @desktop @tablet @mobile
Feature: User Management
  As an Audiotheque admin
  I want to add, remove, and reset passwords for other users
  So that I can give household members their own accounts without giving them shell access

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in

  Scenario: Admin creates a new user
    When User creates new user "bob" with password "bobpass1234"
    Then User "bob" can authenticate with password "bobpass1234"

  Scenario: Admin deletes a user
    Given User "bob" exists with password "bobpass1234"
    When User deletes user "bob"
    Then User "bob" cannot authenticate with password "bobpass1234"

  Scenario: Admin resets another user's password
    Given User "bob" exists with password "bobpass1234"
    When User resets password for "bob" to "newbobpass1234"
    Then User "bob" can authenticate with password "newbobpass1234"
    And User "bob" cannot authenticate with password "bobpass1234"
