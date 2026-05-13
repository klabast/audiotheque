@desktop @tablet @mobile @wip
Feature: MPD Device Discovery
  As a user
  I want Audiotheque to discover my MPD devices
  So that I can easily play music on them

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And User "alice" is logged in

  Scenario: MPD devices are discovered automatically
    Given MPD device "Mock MPD E2E" is on the network
    When User opens device picker
    Then Device list shows "Mock MPD E2E"

  Scenario: Multiple MPD devices are discovered
    Given MPD device "Mock MPD E2E" is on the network
    And MPD device "Kitchen" is on the network
    When User opens device picker
    Then Device list shows "Mock MPD E2E"
    And Device list shows "Kitchen"

  Scenario: MPD device going offline is reflected
    Given MPD device "Mock MPD E2E" is on the network
    And Device list shows "Mock MPD E2E"
    When "Mock MPD E2E" goes offline
    Then Device list shows "Mock MPD E2E" as unavailable

  Scenario: MPD device coming online is detected
    Given MPD device "Mock MPD E2E" is offline
    When "Mock MPD E2E" comes online
    Then Device list shows "Mock MPD E2E" as available

  Scenario: Manual MPD configuration
    Given No MPD devices are discovered
    When User adds MPD device manually with host "192.168.1.100" port "6600"
    Then Device list shows manually configured device
    And Device can be used for playback

  Scenario: Device names can be customized
    Given MPD device "mpd@192.168.1.100" is discovered
    When User renames device to "Mock MPD E2E Speaker"
    Then Device list shows "Mock MPD E2E Speaker"
