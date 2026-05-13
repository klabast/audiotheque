package settings

import (
	"errors"
	"fmt"

	"github.com/google/uuid"
)

const (
	settingStreamingHostname = "streaming_hostname"
	defaultStreamingHostname = "localhost:8080"
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
