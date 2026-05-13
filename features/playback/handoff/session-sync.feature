@desktop @tablet @mobile
Feature: Session Sync
  As a user with multiple devices
  I want all my devices to show the same playback state
  So that I can control playback from any device

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available

  Scenario: New browser sees current playback state
    Given Music is playing on "Mock MPD E2E"
    When User opens Audiotheque in a new browser
    Then New browser shows current track
    And New browser shows "Mock MPD E2E"
    And New browser can control playback

  Scenario: Play action syncs to all clients
    Given User is playing album "Complete Test Album" in browser
    And User has multiple browsers open
    And Music is paused
    When User resumes playback on one browser
    Then All browsers show playing state

  Scenario: Skip action syncs to all clients
    Given User is playing album "Complete Test Album" in browser
    And User has multiple browsers open
    When User skips track on one browser
    Then All browsers show new track

  @wip
  Scenario: Queue changes sync to all clients
    Given User has multiple browsers open
    When User adds track to queue on one browser
    Then All browsers show updated queue

  Scenario: Volume changes sync to all clients
    Given User is playing album "Complete Test Album" in browser
    And User has multiple browsers open
    When User changes volume on one browser
    Then All browsers show new volume level

  Scenario: Device change syncs to all clients
    Given User is playing album "Complete Test Album" in browser
    And User has multiple browsers open
    And Music is playing in browser A
    When User transfers to MPD from browser B
    Then Browser A shows "Mock MPD E2E"
    And Browser B shows "Mock MPD E2E"

  @wip
  Scenario: Progress updates in real-time
    Given User is playing album "Complete Test Album" in browser
    And User has multiple browsers open
    And Music is playing
    Then Progress bar updates on all browsers
    And Current time updates on all browsers

  @wip
  Scenario: Offline client reconnects and syncs
    Given User has browser that went offline
    When Browser reconnects
    Then Browser syncs to current playback state
    And Browser shows current track and position
