@desktop @tablet @mobile
Feature: Queue Sidebar
  As a user
  I want to open the queue sidebar
  So that I can see what is queued and what plays next

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete

  Scenario: User opens the queue sidebar
    When User opens queue sidebar
    Then Queue sidebar is visible

  Scenario: User closes the queue sidebar
    Given Queue sidebar is open
    When User closes queue sidebar
    Then Queue sidebar is hidden

  Scenario: Queue sidebar lists upcoming tracks while album plays
    Given User plays album "Complete Test Album"
    When User opens queue sidebar
    Then Queue sidebar lists upcoming tracks
