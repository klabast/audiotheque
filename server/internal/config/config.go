package config

import "os"

// DefaultDataDir is the default data directory when AUDIOD_DATA_DIR is not set
const DefaultDataDir = "./data"

// DefaultPort is the default HTTP port when AUDIOD_PORT is not set
const DefaultPort = "8080"

// GetDataDir returns the data directory path from AUDIOD_DATA_DIR env var,
// falling back to DefaultDataDir ("./data") if not set.
func GetDataDir() string {
	dataDir := os.Getenv("AUDIOD_DATA_DIR")
	if dataDir == "" {
		return DefaultDataDir
	}
	return dataDir
}

// GetPort returns the HTTP port from AUDIOD_PORT env var,
// falling back to DefaultPort ("8080") if not set.
func GetPort() string {
	port := os.Getenv("AUDIOD_PORT")
	if port == "" {
		return DefaultPort
	}
	return port
}
