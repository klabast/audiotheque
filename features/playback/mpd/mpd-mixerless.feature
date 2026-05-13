@desktop @tablet @mobile
Feature: MPD device without volume control
  As a user with a HiFiBerry-style speaker (MPD `mixer_type "none"`)
  I want the player to tell me clearly that volume can't be controlled
  And I want the volume slider greyed out instead of fighting me
  So that I'm not chasing a 5xx error every time I touch the slider

  Background:
    Given System is in initial state
    And Admin-User "alice" exists with password "alicepass123"
    And Library "Test Music" exists with path "e2e/data/music"
    And User "alice" is logged in
    And Library "Test Music" scan is complete
    And MPD device "Mock MPD E2E" is available
    And "Mock MPD E2E" has no mixer
    And Music is playing on "Mock MPD E2E"

  Scenario: Mixerless device surfaces the capability hint
    When User attempts to change volume to 50% on the mixerless device
    Then Session reports volume capability disabled for current device
    And Session remembers volume 50% for "Mock MPD E2E"
    And Volume control is disabled in the player
