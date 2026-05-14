package settings

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const (
	settingStreamingHostname = "streaming_hostname"
	defaultStreamingHostname = "localhost:8080"

	settingAuthEnabled = "auth_enabled"
)

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) CreateDevice(name, deviceType, address string) (*Device, error) {
	device := &Device{
		ID:      uuid.New().String(),
		Name:    name,
		Type:    deviceType,
		Address: address,
	}
	if err := s.repo.CreateDevice(device); err != nil {
		return nil, fmt.Errorf("failed to create device: %w", err)
	}
	return device, nil
}

func (s *Service) ListDevices() ([]Device, error) {
	return s.repo.ListDevices()
}

func (s *Service) UpdateDevice(id, name, address string) (*Device, error) {
	existing, err := s.repo.GetDevice(id)
	if err != nil {
		return nil, err
	}
	existing.Name = name
	existing.Address = address
	if err := s.repo.UpdateDevice(existing); err != nil {
		return nil, fmt.Errorf("failed to update device: %w", err)
	}
	return existing, nil
}

func (s *Service) DeleteDevice(id string) error {
	return s.repo.DeleteDevice(id)
}

func (s *Service) GetStreamingHostname() (string, error) {
	hostname, err := s.repo.GetSetting(settingStreamingHostname)
	if errors.Is(err, ErrSettingNotFound) {
		return defaultStreamingHostname, nil
	}
	if err != nil {
		return "", err
	}
	return hostname, nil
}

func (s *Service) SetStreamingHostname(hostname string) error {
	return s.repo.SetSetting(settingStreamingHostname, hostname)
}

// IsAuthEnabled reports whether browser authentication is required for this
// instance. Defaults to true when the setting row is missing — i.e. fresh
// installs and existing deployments behave as before until an admin explicitly
// turns auth off. Any value other than the literal string "false" is treated
// as enabled, so a fat-fingered row never accidentally disables auth.
func (s *Service) IsAuthEnabled() (bool, error) {
	v, err := s.repo.GetSetting(settingAuthEnabled)
	if errors.Is(err, ErrSettingNotFound) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return v != "false", nil
}

// SetAuthEnabled persists the auth_enabled toggle. Pairs with IsAuthEnabled —
// see the doc comment there for the default-on / fail-safe semantics.
func (s *Service) SetAuthEnabled(enabled bool) error {
	value := "true"
	if !enabled {
		value = "false"
	}
	return s.repo.SetSetting(settingAuthEnabled, value)
}
