@desktop @tablet @mobile
Feature: No Library State
  As an Audiotheque user
  When no libraries are configured
  I want to see a helpful message with a link to library settings
  So that I can easily set up my first library

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And No libraries exist

  Scenario: User sees no library message
    And User "alice" is logged in
    When User navigates to home page
    Then No-library message is visible
    And Library settings link is visible

  Scenario: Admin user clicks library settings link
    And User "alice" is logged in
    When User navigates to home page
    And User clicks library settings link
    Then User is on library settings page

  Scenario: Non-admin user sees no library message without action button
    Given User "bob" exists with password "bobpass123"
    And User "bob" is logged in
    When User navigates to home page
    Then No-library message is visible
    And Library settings link is not visible

  Scenario: Non-admin user has read-only access to library settings
    Given User "bob" exists with password "bobpass123"
    And User "bob" is logged in
    When User navigates to library settings page
    Then Library settings page is read-only
