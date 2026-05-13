# Audiotheque Server (`audiod`) — Backend Implementation

## Overview

The Audiotheque server is a high-performance Go backend for personal music streaming, designed with simplicity, security, and real-time capabilities at its core. The binary is named `audiod` to follow the daemon-naming convention of MPD, PulseAudio, etc.

## Core Philosophy

Audiotheque is built for **music lovers who value privacy, control, and audio quality**. Unlike commercial streaming services or complex media servers, Audiotheque focuses exclusively on music with:

- **Self-hosted simplicity** - Single binary, SQLite database, zero external dependencies
- **Hi-res audio first** - Native support for FLAC, ALAC, and lossless formats
- **Real-time sync** - WebSocket-based state synchronization across all devices
- **Secure by default** - HttpOnly cookies, Argon2id password hashing, XSS/CSRF protection
- **Personal scale** - Optimized for household music libraries (thousands to tens of thousands of tracks)

## Current Implementation Status

### ✅ Completed Features

**Authentication & Security**:
- [x] HttpOnly cookie-based authentication
- [x] JWT token generation with Argon2id password hashing
- [x] CSRF protection via SameSite=Lax cookies
- [x] Remember Me functionality (7 days vs 1 year cookie expiry)
- [x] Admin user management
- [x] Setup wizard for initial configuration

**Playback System**:
- [x] Spotify-style playback model (one active device per user)
- [x] Real-time WebSocket state synchronization
- [x] Multi-tab safety (audio output from single tab only)
- [x] Device handover with automatic pause
- [x] Unified playback state API
- [x] Progress tracking and broadcasting

**Music Library**:
- [x] Efficient music scanning with metadata extraction
- [x] SQLite database with WAL mode
- [x] Album, artist, and track browsing
- [x] Server-Sent Events (SSE) for real-time library updates
- [x] Hi-res audio detection and tagging
- [x] Multiple library support

**MPD Integration**:
- [x] Automatic mDNS server discovery
- [x] Manual server configuration
- [x] Queue synchronization to MPD
- [x] MPD status monitoring
- [x] Bidirectional playback control

**Audio Processing**:
- [x] FFmpeg-based transcoding
- [x] Intelligent transcode caching
- [x] Multiple format support (MP3, AAC, FLAC, ALAC, etc.)
- [x] Streaming with proper content-type headers

### 🚧 In Progress

**Queue Management**:
- [ ] **WebSocket queue commands** (replacing REST endpoints)
- [ ] Real-time queue updates across all clients
- [ ] Queue operation broadcasts via WebSocket

### 📋 Planned Features

**Mobile & Sync**:
- [ ] Offline sync API for mobile clients
- [ ] Differential sync engine
- [ ] Progressive download for large libraries

**Social Features** (optional):
- [ ] User-specific play history
- [ ] Favorites and ratings
- [ ] Playlist management
- [ ] Playlist sharing

**Advanced Playback**:
- [ ] Crossfade support
- [ ] Gapless playback
- [ ] ReplayGain/volume normalization
- [ ] Smart shuffle algorithms

## Technology Stack

### Core Dependencies

- **Go 1.21+**: Modern, performant backend language
- **[modernc.org/sqlite](https://gitlab.com/cznic/sqlite)**: Pure Go SQLite driver
- **[goose](https://github.com/pressly/goose)**: Database migrations
- **[gorilla/websocket](https://github.com/gorilla/websocket)**: WebSocket implementation
- **[golang-jwt](https://github.com/golang-jwt/jwt)**: JWT token handling
- **FFmpeg**: Audio transcoding (system dependency)

### Architecture Patterns

**Single Connection SQLite**:
- Eliminates SQLITE_BUSY errors entirely
- WAL mode provides excellent concurrent read performance
- Simplified error handling and transaction management
- Perfect for personal-scale deployments

**Repository Pattern**:
- Clean separation between data access and business logic
- Testable service layer with mocked repositories
- Consistent error handling across all data operations

**WebSocket Hub Pattern**:
- Centralized message broadcasting
- Per-device connection management
- Efficient message routing to authenticated users

## Project Structure

```
server/
├── cmd/
│   └── server/          # Main entry point
├── internal/
│   ├── auth/           # Authentication & JWT
│   │   ├── service.go  # Business logic
│   │   ├── handler.go  # HTTP handlers
│   │   ├── jwt.go      # Token generation/validation
│   │   └── password.go # Argon2id hashing
│   ├── playback/       # Playback & queue management
│   │   ├── service.go  # Playback business logic
│   │   ├── hub.go      # WebSocket hub
│   │   ├── websocket.go # WebSocket handlers
│   │   └── handlers.go # REST API handlers
│   ├── library/        # Music library scanning
│   │   ├── scanner.go  # File system scanning
│   │   ├── metadata.go # Audio metadata extraction
│   │   └── repository.go # Database operations
│   ├── mpd/            # MPD integration
│   │   ├── client.go   # MPD client
│   │   ├── discovery.go # mDNS discovery
│   │   └── sync.go     # Queue sync logic
│   ├── transcode/      # Audio transcoding
│   │   ├── service.go  # FFmpeg wrapper
│   │   └── cache.go    # Transcode caching
│   ├── database/       # Database layer
│   │   └── db.go       # SQLite wrapper
│   └── middleware/     # HTTP middleware
│       └── user_context.go # JWT middleware
└── migrations/         # Database schema

```

## API Documentation

### Interactive Docs

When the server is running, access Swagger UI at:
```
http://localhost:8080/api/docs/
```

### OpenAPI Specification

The complete API specification is available at:
- **JSON**: `/docs/swagger.json`
- **YAML**: `/docs/swagger.yaml`

Auto-generated from code annotations using [swag](https://github.com/swaggo/swag).

To regenerate after API changes:
```bash
cd server
swag init -g cmd/server/main.go -o docs
```

### API Organization

**Authentication** (`/api/auth/*`):
- Login, logout, user management
- Setup wizard for initial configuration

**Playback** (`/api/playback/*`):
- WebSocket connection (`/ws`)
- Unified state endpoint
- Queue operations (REST, migrating to WebSocket)
- Playback controls

**Library** (`/api/libraries/*`):
- Library management
- Scanning operations
- SSE streams for real-time updates

**Media** (`/api/albums/*`, `/api/tracks/*`):
- Browse albums and tracks
- Stream audio files
- Transcode on-the-fly

**MPD** (`/api/mpd/*`):
- Server discovery and configuration
- Queue synchronization
- Status monitoring

## Development

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./internal/playback

# With verbose output
go test -v ./internal/auth

# With coverage
go test -cover ./...
```

### Building

```bash
# Development build
go build -o audiod cmd/server/main.go

# Production build with optimizations
go build -ldflags="-s -w" -o audiod cmd/server/main.go
```

### Running Locally

```bash
# Set environment variables
export MUSIC_LIBRARY_PATH=./music
export AUDIOD_DATA_DIR=./data

# Run the server
./audiod
```

Server starts on `http://localhost:8080` by default.

### Database Migrations

Migrations run automatically on startup. To manage manually:

```bash
# Check status
goose -dir migrations sqlite3 ../data/audiod.db status

# Apply all pending migrations
goose -dir migrations sqlite3 ../data/audiod.db up

# Rollback one migration
goose -dir migrations sqlite3 ../data/audiod.db down

# Create new migration
goose -dir migrations create add_new_feature sql
```

## Testing Strategy

### Unit Tests
- Service layer with mocked repositories
- Authentication logic and JWT validation
- Metadata extraction and scanning

### Integration Tests
- Database operations with real SQLite
- MPD connection and synchronization
- WebSocket message handling

### BDD Tests
- Comprehensive playback scenarios
- Multi-tab behavior verification
- Device handover flows

See `CLAUDE.md` for TDD requirements and best practices.

## Performance Characteristics

**Benchmarks** (MacBook Pro M1, 50k track library):

- Library scan: ~2000 tracks/second
- Album browse (SSE): ~1ms per album with full metadata
- WebSocket message broadcast: <1ms to 10 connected clients
- Transcode cache hit: ~0.5ms
- Transcode cache miss (MP3 320kbps): ~2-5 seconds for 3-minute track

**Memory Usage**:
- Baseline: ~50MB
- With 10 WebSocket connections: ~60MB
- During library scan: ~100-150MB
- Transcode cache (100 files): ~500MB disk

## Security Considerations

**Authentication**:
- HttpOnly cookies prevent XSS token theft
- Argon2id provides strong password protection
- Constant-time password comparison prevents timing attacks
- Setup tokens expire after 1 hour

**WebSocket**:
- Authenticated via same JWT middleware as HTTP
- User ID extracted from token, never trusted from client
- Device ID derived from authenticated user ID

**Production Recommendations**:
- Enable HTTPS and set `Secure` cookie flag
- Make JWT secret env var required (currently optional with generated fallback)
- Restrict WebSocket origins to known domains
- Add rate limiting to login endpoint

## Non-Goals

To maintain simplicity and focus, the server intentionally avoids:

- **Video support** - Music only
- **Complex metadata editing** - Use MusicBrainz Picard or similar
- **UPnP/DLNA** - Unreliable protocols, MPD is the preferred network player
- **User social features** - Personal music server, not a social network
- **Distributed deployment** - Single server per household
- **Multiple database backends** - SQLite is perfect for this use case

## Contributing

Contributions are welcome! Please ensure:

1. **Tests pass**: `go test ./...` must succeed
2. **Follow TDD**: Write tests first (see `CLAUDE.md`)
3. **Update docs**: Keep API documentation current
4. **Run linters**: `golangci-lint run`
5. **Meaningful commits**: Clear, descriptive commit messages

## License

AGPL v3 — see the top-level [LICENSE](../LICENSE) file for the full text.
