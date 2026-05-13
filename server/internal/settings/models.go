package settings

import (
	"errors"
	"time"
)

var (
	ErrDeviceNotFound  = errors.New("device not found")
	ErrSettingNotFound = errors.New("setting not found")
)

type Device struct {
	ID        string
	Name      string
	Type      string // "mpd"
	Address   string // "192.168.1.10:6600"
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Repository interface {
	CreateDevice(device *Device) error
	GetDevice(id string) (*Device, error)
	ListDevices() ([]Device, error)
	UpdateDevice(device *Device) error
	DeleteDevice(id string) error
	GetSetting(key string) (string, error)
	SetSetting(key, value string) error
}
