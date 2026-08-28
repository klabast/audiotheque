package commands

import (
	"audiod/internal/auth"
	"audiod/internal/branding"
	"audiod/internal/config"
	"audiod/internal/database"
	"audiod/internal/jobs"
	"audiod/internal/library"
	"audiod/internal/playback"
	"audiod/internal/scanner"
	"audiod/internal/server"
	"audiod/internal/settings"
	"audiod/internal/system"
	"audiod/internal/websocket"
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

var logLevel string

var serverCmd = &cobra.Command{
	Use:   "server",
	Short: "Start the " + branding.AppName + " HTTP server",
	Long:  "Start the " + branding.AppName + " HTTP server to serve the web UI and API.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return startServer()
	},
}

func init() {
	serverCmd.Flags().StringVar(&logLevel, "log-level", "info", "log level (debug, info, warn, error)")
}

func parseLogLevel(level string) slog.Level {
	switch strings.ToLower(level) {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

func startServer() error {
	// Initialize structured logger
	logOpts := &slog.HandlerOptions{Level: parseLogLevel(logLevel)}
	logger := slog.New(slog.NewTextHandler(os.Stdout, logOpts))
	slog.SetDefault(logger)

	// Initialize database
	db, err := database.Open()
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Initialize auth (with DB-backed session store for browser sessions).
	// Variable name is "authSessionRepo" to disambiguate from the playback
	// SessionRepository defined further down — they're separate concerns.
	authRepo := auth.NewRepository(db)
	authSessionRepo := auth.NewSessionRepository(db)
	authService := auth.NewService(authRepo, authSessionRepo)
	authHandler := auth.NewHandler(authService)

	// Initialize system
	systemHandler := system.NewHandler(authService)

	// Initialize WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Initialize library
	libraryRepo := library.NewRepository(db)
	libraryService := library.NewService(libraryRepo)
	libraryHandler := library.NewHandler(libraryService, authService)

	// Cover thumbnailer: scaled, on-disk-cached covers for the library grid.
	// Mobile devices stutter on full-size cover bursts; the cached thumb
	// (≈400px JPEG quality 85) is roughly an order of magnitude smaller.
	thumbCacheDir := filepath.Join(config.GetDataDir(), "cache", "covers")
	libraryHandler.SetThumbnailer(library.NewCoverThumbnailer(thumbCacheDir))

	// One-time pass to populate sort_name on artists scanned before the
	// article-stripping logic existed. Idempotent — runs every startup, only
	// touches rows where sort_name is NULL/empty.
	if n, err := libraryRepo.BackfillArtistSortNames(); err != nil {
		logger.Warn("artist sort_name backfill failed", "error", err)
	} else if n > 0 {
		logger.Info("backfilled artist sort_name", "rows", n)
	}

	// Initialize scanner worker (background job that processes scan queue)
	scannerWorker := scanner.NewWorker(libraryRepo, hub)
	scannerWorker.Start()
	defer scannerWorker.Stop()

	// Initialize job scheduler
	scheduler := jobs.NewScheduler()

	// Register auth domain jobs
	scheduler.Register(auth.NewResetCodeCleanupJob(authService))

	// Start background jobs
	scheduler.Start()
	defer scheduler.Stop()

	// Initialize playback. Sessions persist in DB so state survives restarts.
	sessionRepo := playback.NewDBSessionRepository(db)
	trackProvider := playback.NewLibraryTrackProvider(libraryService)
	playbackService := playback.NewService(trackProvider, sessionRepo)

	// Migration: under the unified-session invariant every session names a
	// real playback device. Sweep any pre-invariant rows so they don't
	// resurface in GetSession with an empty DeviceID. v2 deploys are fresh
	// per the project's TrueNAS plan; surviving rows are noise to discard.
	if removed, err := sessionRepo.DeleteWithoutDevice(); err != nil {
		logger.Warn("startup migration: delete sessions without device failed", "error", err)
	} else if removed > 0 {
		logger.Info("startup migration: purged sessions without device", "count", removed)
	}

	// Initialize settings
	settingsRepo := settings.NewRepository(db)
	settingsService := settings.NewService(settingsRepo)
	settingsHandler := settings.NewHandler(settingsService, authService)

	// Wire the auth-enabled lookup so middleware can short-circuit when an
	// admin has turned login off. Kept as a setter (not a constructor arg) so
	// the auth package has no compile-time dependency on settings.
	authService.SetAuthEnabledFn(settingsService.IsAuthEnabled)

	// Initialize device registry (DB-backed for production)
	deviceRegistry := settings.NewDBDeviceRegistry(settingsRepo)

	// Ephemeral browser-tab registry: WebSocket connections are auto-registered
	// here so other tabs can address them as transferable devices.
	browserRegistry := playback.NewBrowserDeviceRegistry()

	// Wire production device resolver — knows about MPD devices and (via the
	// browser registry) live browser tabs.
	prodResolver := playback.NewRegistryDeviceResolver(deviceRegistry, logger)
	prodResolver.SetBrowserRegistry(browserRegistry)
	playbackService.SetDeviceResolver(prodResolver)
	playbackService.SetLogger(logger)

	// Wire WS broadcaster: every session mutation pushes to user's clients.
	// exceptClientID lets us skip the originating client (e.g. the browser
	// that just sent its position tick) so it doesn't get its own value back.
	// The broadcaster reuses the same capabilitiesFn shape as the HTTP
	// handler so push-broadcasts and REST responses surface the same hint.
	wsCaps := func(deviceID string) *playback.DeviceCapabilities {
		device, err := prodResolver.ResolveDevice(deviceID)
		if err != nil {
			return nil
		}
		caps := &playback.DeviceCapabilities{Volume: true}
		if cap, ok := device.(interface{ SupportsVolume() bool }); ok {
			caps.Volume = cap.SupportsVolume()
		}
		return caps
	}
	playbackService.SetBroadcaster(playback.SessionBroadcasterFunc(func(userID int64, session *playback.Session, exceptClientID string) {
		msg := websocket.Message{
			Type: "playback-session",
			Data: playback.SessionToResponse(session, wsCaps),
		}
		if err := hub.BroadcastToUserExcept(userID, exceptClientID, msg); err != nil {
			logger.Error("broadcast session failed", "userID", userID, "error", err)
		}
	}))

	// Poll MPD-bound sessions for position changes. Browser sessions report
	// position over WS; MPD doesn't push, so we poll the device.
	pollCtx, cancelPoll := context.WithCancel(context.Background())
	defer cancelPoll()
	playbackService.StartMPDPolling(pollCtx, 1*time.Second)

	// Stream URL builder reads hostname from settings and signs the URL with a
	// short-lived JWT so MPD devices (which have no session cookie) can fetch
	// the stream. The token is bound to (trackID, userID) so a captured URL
	// only exposes that one track for that user.
	port := config.GetPort()
	const streamTokenTTL = 4 * time.Hour
	playbackService.SetStreamURLBuilder(func(trackID, userID int64) string {
		hostname, err := settingsService.GetStreamingHostname()
		if err != nil {
			hostname = "localhost:" + port
		}
		base := fmt.Sprintf("http://%s/api/tracks/%d/stream", hostname, trackID)
		token, err := auth.MintStreamToken(trackID, userID, streamTokenTTL)
		if err != nil {
			logger.Error("mint stream token failed", "trackID", trackID, "userID", userID, "error", err)
			return base
		}
		return base + "?token=" + token
	})

	// Wire WS incoming messages to playback service. clientID identifies the
	// sender so the resulting broadcast can skip them.
	hub.SetMessageHandler(func(userID int64, clientID string, msg websocket.Message) {
		if msg.Type == "playback-position" {
			if data, ok := msg.Data.(map[string]interface{}); ok {
				if pos, ok := data["position"].(float64); ok {
					playbackService.UpdatePositionFromClient(userID, int(pos), clientID)
				}
			}
		}
	})

	// Hub lifecycle hooks: register/unregister browser tabs as ephemeral
	// devices, and tell the freshly-connected client what its hub-assigned
	// client ID is so it can send X-Audiod-Client-Id on REST calls and recognise
	// itself in the device list.
	hub.SetLifecycleHandlers(
		func(c *websocket.Client) {
			browserRegistry.Register(c.ID, c.UserID, playback.DeviceNameFromUA(c.UserAgent))
			if err := hub.SendToClient(c.ID, websocket.Message{
				Type: "client-id",
				Data: map[string]string{"clientId": c.ID},
			}); err != nil {
				logger.Warn("send client-id failed", "clientID", c.ID, "error", err)
			}
		},
		func(c *websocket.Client) {
			browserRegistry.Unregister(c.ID)
		},
	)

	// Create user ID getter that uses auth middleware
	getUserID := func(r *http.Request) (int64, error) {
		user, err := auth.GetAuthenticatedUser(r, authService)
		if err != nil {
			return 0, errors.New("unauthorized")
		}
		return user.ID, nil
	}
	playbackHandler := playback.NewHandler(playbackService, getUserID)

	// Set track store for streaming (library repo implements TrackStore interface)
	playbackHandler.SetTrackStore(libraryRepo)
	playbackHandler.SetDeviceRegistry(deviceRegistry)
	playbackHandler.SetBrowserRegistry(browserRegistry)
	playbackHandler.SetDeviceResolver(prodResolver)
	// Accept ?token= as a fallback when cookie auth fails — the path used by
	// MPD devices fetching signed stream URLs.
	playbackHandler.SetStreamTokenValidator(auth.ValidateStreamToken)

	// Initialize server
	srv := server.New(authHandler, systemHandler, libraryHandler, playbackHandler, settingsHandler, hub)

	// Wire WS auth so upgrades register clients under the authenticated user.
	// Without this, BroadcastToUserExcept matches no clients (all userID=0)
	// and live updates never reach other tabs.
	srv.SetUserIDGetter(websocket.UserIDGetter(getUserID))

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start HTTP server in goroutine
	addr := ":" + port
	go func() {
		logger.Info("starting "+branding.AppName+" server", "addr", addr, "logLevel", logLevel)
		if err := http.ListenAndServe(addr, srv.Router()); err != nil {
			log.Fatal(err)
		}
	}()

	// Wait for shutdown signal
	<-stop
	logger.Info("shutting down gracefully")
	return nil
}
