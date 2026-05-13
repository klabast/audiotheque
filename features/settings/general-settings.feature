@desktop @tablet @mobile
Feature: General Settings
  As a logged-in Audiotheque user
  I want to customize my application preferences
  So that I can personalize my experience

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in

  Scenario: User switches to dark theme
    When User changes theme to "dark"
    Then App displays in dark theme

  Scenario: User switches to light theme
    When User changes theme to "light"
    Then App displays in light theme

  Scenario: User sets theme to follow system
    When User changes theme to "system"
    Then App theme reflects system theme

  Scenario: User changes language to German
    When User changes language to "de"
    Then App displays in German language

  Scenario: User switches language from German back to English
    When User changes language to "de"
    Then App displays in German language
    When User changes language to "en"
    Then App displays in English language
