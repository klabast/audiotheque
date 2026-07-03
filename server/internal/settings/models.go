package settings

import (
	"errors"
	"time"
)

var (
	ErrDeviceNotFound  = errors.New("device not found")
	ErrSettingNotFound = errors.New("setting not found")
)

// json tags mirror Go's default (untagged) marshaling; they exist so the
// swagger spec matches the actual wire format instead of swag's camelCase
// property strategy. Don't lowercase them without migrating the UI.
type Device struct {
	ID        string    `json:"ID"`
	Name      string    `json:"Name"`
	Type      string    `json:"Type"`    // "mpd"
	Address   string    `json:"Address"` // "192.168.1.10:6600"
	CreatedAt time.Time `json:"CreatedAt"`
	UpdatedAt time.Time `json:"UpdatedAt"`
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
