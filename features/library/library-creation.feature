@desktop @tablet @mobile
Feature: Library Creation
  As an admin user
  I want to create a music library
  So that I can organize and access my music collection

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in
    And No libraries exist

  Scenario: Admin creates first library with test music
    When User creates library named "Test Music" with path "e2e/data/music"
    Then Library "Test Music" exists
    And Library scanning completes successfully
    And Library is browsable
