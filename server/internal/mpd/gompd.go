package mpd

import (
	"fmt"
	"strconv"

	gompdlib "github.com/fhs/gompd/v2/mpd"
)

// GompdClient wraps github.com/fhs/gompd/v2/mpd to implement the Client interface
type GompdClient struct {
	conn *gompdlib.Client
}

// Dial connects to an MPD server at the given address (e.g., "localhost:6600")
func Dial(addr string) (*GompdClient, error) {
	conn, err := gompdlib.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("mpd dial: %w", err)
	}
	return &GompdClient{conn: conn}, nil
}

func (c *GompdClient) Play() error {
	return c.conn.Play(-1)
}

func (c *GompdClient) Pause() error {
	return c.conn.Pause(true)
}

func (c *GompdClient) Stop() error {
	return c.conn.Stop()
}

func (c *GompdClient) Status() (Status, error) {
	attrs, err := c.conn.Status()
	if err != nil {
		return Status{}, fmt.Errorf("mpd status: %w", err)
	}

	volume, _ := strconv.Atoi(attrs["volume"])
	// MPD reports elapsed with sub-second precision (e.g. "12.345"). Parse as
	// float and truncate to int seconds — the rest of the codebase carries
	// position as int seconds and truncation keeps the broadcast value pinned
	// to "the second the user just heard", never rounding forward past it.
	elapsedFloat, _ := strconv.ParseFloat(attrs["elapsed"], 64)
	elapsed := int(elapsedFloat)

	return Status{
		State:   attrs["state"],
		Elapsed: elapsed,
		Volume:  volume,
		SongID:  attrs["songid"], // empty when MPD is fully idle
	}, nil
}

func (c *GompdClient) CurrentSong() (CurrentSong, error) {
	attrs, err := c.conn.CurrentSong()
	if err != nil {
		return CurrentSong{}, fmt.Errorf("mpd currentsong: %w", err)
	}
	return CurrentSong{
		File: attrs["file"],
	}, nil
}

func (c *GompdClient) SetVolume(volume int) error {
	return c.conn.SetVolume(volume)
}

func (c *GompdClient) Seek(position int) error {
	// Seek current song to position
	return c.conn.Command("seekcur %d", position).OK()
}

func (c *GompdClient) LoadURL(url string) error {
	if err := c.conn.Clear(); err != nil {
		return fmt.Errorf("mpd clear: %w", err)
	}
	if err := c.conn.Add(url); err != nil {
		return fmt.Errorf("mpd add: %w", err)
	}
	return nil
}

func (c *GompdClient) Close() error {
	return c.conn.Close()
}
