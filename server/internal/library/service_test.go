package library

import (
	"strings"
	"testing"
	"time"
)

// testUserID is the user the mock repository grants library access to.
// mockRepository keys libraries by user, so seeding repo.libraries[testUserID]
// is what makes UserHasLibraryAccess return true.
const testUserID int64 = 1

// Mock repository for testing
type mockRepository struct {
	libraries map[int64][]*Library // userID -> libraries
	artists   map[int64]*Artist
	albums    map[int64]*Album
	tracks    map[int64]*Track
	scanJobs  map[int64]*ScanJob // libraryID -> job
	nextJobID int64
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		libraries: make(map[int64][]*Library),
		artists:   make(map[int64]*Artist),
		albums:    make(map[int64]*Album),
		tracks:    make(map[int64]*Track),
		scanJobs:  make(map[int64]*ScanJob),
		nextJobID: 1,
	}
}

func (m *mockRepository) ListLibrariesForUser(userID int64) ([]*Library, error) {
	libs := m.libraries[userID]
	if libs == nil {
		return []*Library{}, nil
	}
	return libs, nil
}

func (m *mockRepository) CreateLibrary(name string, paths []string) (*Library, error) {
	// Mock implementation - returns a library with ID 1
	library := &Library{
		ID:    1,
		Name:  name,
		Paths: paths,
	}
	return library, nil
}

func (m *mockRepository) GrantAccess(userID, libraryID int64) error {
	// Mock implementation
	return nil
}

func (m *mockRepository) RevokeAccess(userID, libraryID int64) error {
	// Mock implementation
	return nil
}

func (m *mockRepository) GetLibraryByID(libraryID int64) (*Library, error) {
	// Mock implementation - search for library across all users
	for _, libs := range m.libraries {
		for _, lib := range libs {
			if lib.ID == libraryID {
				return lib, nil
			}
		}
	}
	return nil, ErrLibraryNotFound
}

func (m *mockRepository) DeleteLibrary(libraryID int64) error {
	for userID, libs := range m.libraries {
		for i, lib := range libs {
			if lib.ID == libraryID {
				m.libraries[userID] = append(libs[:i], libs[i+1:]...)
				return nil
			}
		}
	}
	return ErrLibraryNotFound
}

func (m *mockRepository) UpdateLibrary(libraryID int64, name string, paths []string) (*Library, error) {
	for _, libs := range m.libraries {
		for _, lib := range libs {
			if lib.ID == libraryID {
				lib.Name = name
				lib.Paths = paths
				return lib, nil
			}
		}
	}
	return nil, ErrLibraryNotFound
}

// Artist methods
func (m *mockRepository) GetOrCreateArtist(libraryID int64, name string) (*Artist, error) {
	for _, a := range m.artists {
		if a.LibraryID == libraryID && a.Name == name {
			return a, nil
		}
	}
	id := int64(len(m.artists) + 1)
	artist := &Artist{ID: id, LibraryID: libraryID, Name: name, CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.artists[id] = artist
	return artist, nil
}

func (m *mockRepository) UpdateArtistMusicBrainzID(id int64, mbid string) error {
	if artist, ok := m.artists[id]; ok {
		artist.MusicBrainzID = mbid
		return nil
	}
	return ErrArtistNotFound
}

func (m *mockRepository) BackfillArtistSortNames() (int64, error) {
	var n int64
	for _, a := range m.artists {
		if a.SortName == "" {
			a.SortName = computeSortName(a.Name)
			n++
		}
	}
	return n, nil
}

func (m *mockRepository) GetArtistByID(id int64) (*Artist, error) {
	if artist, ok := m.artists[id]; ok {
		return artist, nil
	}
	return nil, ErrArtistNotFound
}

// Album methods
func (m *mockRepository) GetOrCreateAlbum(libraryID int64, artistID *int64, title string, releaseDate string, folderPath string) (*Album, error) {
	for _, a := range m.albums {
		if a.LibraryID == libraryID && a.Title == title && a.FolderPath == folderPath {
			if artistID == nil && a.ArtistID == nil {
				return a, nil
			}
			if artistID != nil && a.ArtistID != nil && *artistID == *a.ArtistID {
				return a, nil
			}
		}
	}
	id := int64(len(m.albums) + 1)
	album := &Album{ID: id, LibraryID: libraryID, ArtistID: artistID, Title: title, ReleaseDate: releaseDate, FolderPath: folderPath, ReleaseType: detectReleaseType(folderPath), CreatedAt: time.Now(), UpdatedAt: time.Now()}
	m.albums[id] = album
	return album, nil
}

func (m *mockRepository) GetAlbumByID(id int64) (*Album, error) {
	if album, ok := m.albums[id]; ok {
		return album, nil
	}
	return nil, ErrAlbumNotFound
}

func (m *mockRepository) UpdateAlbumCoverArt(id int64, coverPath string) error {
	if album, ok := m.albums[id]; ok {
		album.CoverArtPath = coverPath
		return nil
	}
	return ErrAlbumNotFound
}

func (m *mockRepository) ListAlbumsByLibrary(libraryID int64, opts ListAlbumsOptions) ([]*Album, error) {
	var albums []*Album
	for _, a := range m.albums {
		if a.LibraryID != libraryID {
			continue
		}
		if opts.HiResOnly && !a.IsHiRes {
			continue
		}
		albums = append(albums, a)
	}
	return albums, nil
}

func (m *mockRepository) SearchAlbumsByLibrary(libraryID int64, query string, limit int) ([]*Album, error) {
	q := strings.ToLower(query)
	var matches []*Album
	for _, a := range m.albums {
		if a.LibraryID != libraryID {
			continue
		}
		if strings.Contains(strings.ToLower(a.Title), q) {
			matches = append(matches, a)
		}
		if limit > 0 && len(matches) >= limit {
			break
		}
	}
	return matches, nil
}

func (m *mockRepository) SearchArtistsByLibrary(libraryID int64, query string, limit int) ([]*Artist, error) {
	q := strings.ToLower(query)
	var matches []*Artist
	for _, a := range m.artists {
		if a.LibraryID != libraryID {
			continue
		}
		if strings.Contains(strings.ToLower(a.Name), q) {
			matches = append(matches, a)
		}
		if limit > 0 && len(matches) >= limit {
			break
		}
	}
	return matches, nil
}

func (m *mockRepository) SearchTracksByLibrary(libraryID int64, query string, limit int) ([]*Track, error) {
	q := strings.ToLower(query)
	var matches []*Track
	for _, t := range m.tracks {
		if t.LibraryID != libraryID {
			continue
		}
		if strings.Contains(strings.ToLower(t.Title), q) {
			matches = append(matches, t)
		}
		if limit > 0 && len(matches) >= limit {
			break
		}
	}
	return matches, nil
}

// Track methods
func (m *mockRepository) CreateOrUpdateTrack(track *Track) (*Track, error) {
	for _, t := range m.tracks {
		if t.LibraryID == track.LibraryID && t.FilePath == track.FilePath {
			// Update existing
			t.Title = track.Title
			t.ArtistID = track.ArtistID
			t.AlbumID = track.AlbumID
			return t, nil
		}
	}
	id := int64(len(m.tracks) + 1)
	track.ID = id
	m.tracks[id] = track
	return track, nil
}

func (m *mockRepository) GetTrackByID(trackID int64) (*Track, error) {
	if t, ok := m.tracks[trackID]; ok {
		return t, nil
	}
	return nil, ErrTrackNotFound
}

func (m *mockRepository) GetTrackByPath(libraryID int64, filePath string) (*Track, error) {
	for _, t := range m.tracks {
		if t.LibraryID == libraryID && t.FilePath == filePath {
			return t, nil
		}
	}
	return nil, ErrTrackNotFound
}

func (m *mockRepository) UserHasLibraryAccess(userID, libraryID int64) (bool, error) {
	// Check if user has any libraries and if this library is in the list
	libs, ok := m.libraries[userID]
	if !ok {
		return false, nil
	}
	for _, lib := range libs {
		if lib.ID == libraryID {
			return true, nil
		}
	}
	return false, nil
}

func (m *mockRepository) DeleteTracksByPaths(libraryID int64, paths []string) (int64, error) {
	var deleted int64
	for _, p := range paths {
		for id, t := range m.tracks {
			if t.LibraryID == libraryID && t.FilePath == p {
				delete(m.tracks, id)
				deleted++
			}
		}
	}
	return deleted, nil
}

func (m *mockRepository) ListTracksByAlbum(albumID int64) ([]*Track, error) {
	var tracks []*Track
	for _, t := range m.tracks {
		if t.AlbumID != nil && *t.AlbumID == albumID {
			tracks = append(tracks, t)
		}
	}
	return tracks, nil
}

func (m *mockRepository) ListTracksByLibrary(libraryID int64) ([]*Track, error) {
	var tracks []*Track
	for _, t := range m.tracks {
		if t.LibraryID == libraryID {
			tracks = append(tracks, t)
		}
	}
	return tracks, nil
}

func (m *mockRepository) GetTrackPathsForLibrary(libraryID int64) (map[string]time.Time, error) {
	result := make(map[string]time.Time)
	for _, t := range m.tracks {
		if t.LibraryID == libraryID {
			result[t.FilePath] = t.FileModified
		}
	}
	return result, nil
}

// Track-Artist relationship
func (m *mockRepository) AddTrackArtist(trackID, artistID int64, role string, position int) error {
	return nil // Mock - no-op
}

func (m *mockRepository) GetTrackArtists(trackID int64) ([]*TrackArtist, error) {
	return nil, nil // Mock - returns empty
}

// Scan queue methods
func (m *mockRepository) QueueScan(libraryID int64) (*ScanJob, error) {
	job := &ScanJob{
		ID:          m.nextJobID,
		LibraryID:   libraryID,
		Status:      "pending",
		RequestedAt: time.Now(),
	}
	m.nextJobID++
	m.scanJobs[libraryID] = job
	return job, nil
}

func (m *mockRepository) GetPendingScan() (*ScanJob, error) {
	for _, job := range m.scanJobs {
		if job.Status == "pending" {
			return job, nil
		}
	}
	return nil, nil
}

func (m *mockRepository) GetScanJobByLibrary(libraryID int64) (*ScanJob, error) {
	if job, ok := m.scanJobs[libraryID]; ok {
		return job, nil
	}
	return nil, nil
}

func (m *mockRepository) UpdateScanJob(job *ScanJob) error {
	m.scanJobs[job.LibraryID] = job
	return nil
}

func (m *mockRepository) DeleteScanJob(jobID int64) error {
	for libraryID, job := range m.scanJobs {
		if job.ID == jobID {
			delete(m.scanJobs, libraryID)
			return nil
		}
	}
	return nil
}

func (m *mockRepository) ResetOrphanedJobs(timeout time.Duration) (int64, error) {
	// Mock: reset any "running" jobs to "pending"
	var count int64
	cutoff := time.Now().Add(-timeout)
	for _, job := range m.scanJobs {
		if job.Status == "running" && job.UpdatedAt.Before(cutoff) {
			job.Status = "pending"
			job.StartedAt = nil
			job.UpdatedAt = time.Now()
			count++
		}
	}
	return count, nil
}

func (m *mockRepository) ResetAllRunningJobs() (int64, error) {
	var count int64
	for _, job := range m.scanJobs {
		if job.Status == "running" {
			job.Status = "pending"
			job.StartedAt = nil
			job.UpdatedAt = time.Now()
			count++
		}
	}
	return count, nil
}

// TestListLibraries tests that a user can list their accessible libraries
func TestListLibraries(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo) // nil hub is fine for tests not testing WebSocket
	userID := int64(1)

	// Act
	libraries, err := service.ListLibraries(userID)

	// Assert
	if err != nil {
		t.Fatalf("ListLibraries() failed: %v", err)
	}

	if libraries == nil {
		t.Fatal("ListLibraries() returned nil")
	}

	if len(libraries) != 0 {
		t.Errorf("Expected empty library list, got %d libraries", len(libraries))
	}
}

// TestCreateLibrary tests that an admin can create a library with paths
func TestCreateLibrary(t *testing.T) {
	// Arrange - create temp directories for testing
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	repo := newMockRepository()
	service := NewService(repo)
	adminUserID := int64(1)
	name := "My Music"
	paths := []string{tempDir1, tempDir2}

	// Act
	library, err := service.CreateLibrary(adminUserID, name, paths)

	// Assert
	if err != nil {
		t.Fatalf("CreateLibrary() failed: %v", err)
	}

	if library == nil {
		t.Fatal("CreateLibrary() returned nil library")
	}

	if library.Name != name {
		t.Errorf("Expected library name %q, got %q", name, library.Name)
	}

	if len(library.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(library.Paths))
	}

	if library.Paths[0] != tempDir1 {
		t.Errorf("Expected first path %q, got %q", tempDir1, library.Paths[0])
	}

	if library.Paths[1] != tempDir2 {
		t.Errorf("Expected second path %q, got %q", tempDir2, library.Paths[1])
	}

	if library.ID == 0 {
		t.Error("Expected library ID to be set, got 0")
	}
}

// TestCreateLibraryWithoutPaths tests that creating a library without paths fails
func TestCreateLibraryWithoutPaths(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	adminUserID := int64(1)
	name := "My Music"
	paths := []string{} // Empty paths

	// Act
	library, err := service.CreateLibrary(adminUserID, name, paths)

	// Assert
	if err != ErrPathsRequired {
		t.Errorf("Expected ErrPathsRequired, got %v", err)
	}

	if library != nil {
		t.Error("Expected nil library, got non-nil")
	}
}

// ==============================================================================
// New Scanner Tests (TDD - RED phase)
// ==============================================================================

// TestScanLibrary_StartsSuccessfully tests that scan starts without error
func TestScanLibrary_StartsSuccessfully(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)

	library := &Library{
		ID:    libraryID,
		Name:  "Test Music",
		Paths: []string{"/tmp/test-music"},
	}
	repo.libraries[1] = []*Library{library}

	// Act
	err := service.StartScan(libraryID)

	// Assert
	if err != nil {
		t.Fatalf("StartScan() failed: %v", err)
	}
}

// TestStartScan_PreventsConcurrentScans tests that concurrent scans are prevented
func TestStartScan_PreventsConcurrentScans(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)

	library := &Library{
		ID:    libraryID,
		Name:  "Test Music",
		Paths: []string{"/tmp/test-music"},
	}
	repo.libraries[1] = []*Library{library}

	// Act - start first scan
	err1 := service.StartScan(libraryID)
	if err1 != nil {
		t.Fatalf("First StartScan() failed: %v", err1)
	}

	// Act - attempt second scan on same library
	err2 := service.StartScan(libraryID)

	// Assert
	if err2 != ErrScanAlreadyInProgress {
		t.Errorf("Expected ErrScanAlreadyInProgress, got %v", err2)
	}
}

// TestStartScan_AllowsMultipleLibraries tests that different libraries can scan concurrently
func TestStartScan_AllowsMultipleLibraries(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)

	library1 := &Library{ID: 1, Name: "Library 1", Paths: []string{"/tmp/lib1"}}
	library2 := &Library{ID: 2, Name: "Library 2", Paths: []string{"/tmp/lib2"}}
	repo.libraries[1] = []*Library{library1, library2}

	// Act - start scans on different libraries
	err1 := service.StartScan(1)
	err2 := service.StartScan(2)

	// Assert - both should succeed
	if err1 != nil {
		t.Errorf("First StartScan() failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("Second StartScan() failed: %v", err2)
	}
}

// TestGetScanProgress_ReturnsProgress tests getting current scan progress
func TestGetScanProgress_ReturnsProgress(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)

	library := &Library{
		ID:    libraryID,
		Name:  "Test Music",
		Paths: []string{"/tmp/test-music"},
	}
	repo.libraries[1] = []*Library{library}

	// Queue scan (note: in new architecture, jobs start as "pending")
	err := service.StartScan(libraryID)
	if err != nil {
		t.Fatalf("StartScan() failed: %v", err)
	}

	// Act
	progress, err := service.GetScanProgress(testUserID, libraryID)

	// Assert
	if err != nil {
		t.Fatalf("GetScanProgress() failed: %v", err)
	}

	if progress == nil {
		t.Fatal("GetScanProgress() returned nil progress")
	}

	// Jobs start as "pending" until the scanner worker picks them up
	if progress.Status != "pending" {
		t.Errorf("Expected status 'pending', got %q", progress.Status)
	}

	if progress.LibraryID != libraryID {
		t.Errorf("Expected libraryID %d, got %d", libraryID, progress.LibraryID)
	}
}

// TestService_StartScan_AfterOrphanReset_QueuesNewJob documents the user-path
// fix for bug 1 (409 Conflict after container restart):
//
//  1. Before the fix, a stale 'running' scan_queue row with a fresh heartbeat
//     caused StartScan to keep returning ErrScanAlreadyInProgress.
//  2. The worker's boot-time reset (ResetAllRunningJobs) clears the row to
//     'pending'.
//  3. The worker then picks up the pending row and either completes the scan
//     or — once the user re-queues — a new row is queued.
//
// This test walks through that sequence at the service layer.
func TestService_StartScan_AfterOrphanReset_QueuesNewJob(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo)
	const libraryID = int64(1)

	// Pre-restart state: a 'running' row left over from a dead worker. Heartbeat
	// is fresh, so a timeout-based reset would NOT clear it.
	repo.scanJobs[libraryID] = &ScanJob{
		ID:        100,
		LibraryID: libraryID,
		Status:    "running",
		UpdatedAt: time.Now(),
	}
	repo.nextJobID = 101

	// Reproduces the original bug.
	if err := service.StartScan(libraryID); err != ErrScanAlreadyInProgress {
		t.Fatalf("precondition: expected ErrScanAlreadyInProgress, got %v", err)
	}

	// Worker boots: every 'running' row is unconditionally reset to 'pending'.
	n, err := repo.ResetAllRunningJobs()
	if err != nil {
		t.Fatalf("ResetAllRunningJobs: %v", err)
	}
	if n != 1 {
		t.Fatalf("reset count: want 1, got %d", n)
	}

	// Worker then picks up the pending job and completes it (DeleteScanJob on
	// completion). We simulate that final step here.
	if err := repo.DeleteScanJob(100); err != nil {
		t.Fatalf("DeleteScanJob: %v", err)
	}

	// User clicks scan again — must succeed now that the stale row is gone.
	if err := service.StartScan(libraryID); err != nil {
		t.Fatalf("StartScan after orphan reset + worker completion: %v", err)
	}
}

// TestGetScanProgress_NoScanInProgress tests error when no scan is running
func TestGetScanProgress_NoScanInProgress(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)
	repo.libraries[testUserID] = []*Library{{ID: libraryID, Name: "Test"}}

	// Act - no scan started
	progress, err := service.GetScanProgress(testUserID, libraryID)

	// Assert
	if err != ErrNoScanInProgress {
		t.Errorf("Expected ErrNoScanInProgress, got %v", err)
	}

	if progress != nil {
		t.Error("Expected nil progress, got non-nil")
	}
}

// ==============================================================================
// Delete Library Tests
// ==============================================================================

// TestDeleteLibrary tests that a library can be deleted
func TestDeleteLibrary(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)

	library := &Library{
		ID:    libraryID,
		Name:  "Test Music",
		Paths: []string{"/tmp/test-music"},
	}
	repo.libraries[1] = []*Library{library}

	// Act
	err := service.DeleteLibrary(libraryID)

	// Assert
	if err != nil {
		t.Fatalf("DeleteLibrary() failed: %v", err)
	}
}

// TestDeleteLibrary_NotFound tests that deleting non-existent library fails
func TestDeleteLibrary_NotFound(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	nonExistentID := int64(999)

	// Act
	err := service.DeleteLibrary(nonExistentID)

	// Assert
	if err != ErrLibraryNotFound {
		t.Errorf("Expected ErrLibraryNotFound, got %v", err)
	}
}

// ==============================================================================
// Update Library Tests
// ==============================================================================

// TestUpdateLibrary_ChangeName tests that library name can be updated
func TestUpdateLibrary_ChangeName(t *testing.T) {
	// Arrange - create temp directory for testing
	tempDir := t.TempDir()

	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)

	library := &Library{
		ID:    libraryID,
		Name:  "Old Name",
		Paths: []string{tempDir},
	}
	repo.libraries[1] = []*Library{library}

	// Act
	updated, err := service.UpdateLibrary(libraryID, "New Name", []string{tempDir})

	// Assert
	if err != nil {
		t.Fatalf("UpdateLibrary() failed: %v", err)
	}

	if updated.Name != "New Name" {
		t.Errorf("Expected name 'New Name', got %q", updated.Name)
	}
}

// TestUpdateLibrary_ChangePaths tests that library paths can be updated
func TestUpdateLibrary_ChangePaths(t *testing.T) {
	// Arrange - create temp directories for testing
	tempDir1 := t.TempDir()
	tempDir2 := t.TempDir()

	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)

	library := &Library{
		ID:    libraryID,
		Name:  "Music",
		Paths: []string{tempDir1},
	}
	repo.libraries[1] = []*Library{library}

	// Act
	updated, err := service.UpdateLibrary(libraryID, "Music", []string{tempDir1, tempDir2})

	// Assert
	if err != nil {
		t.Fatalf("UpdateLibrary() failed: %v", err)
	}

	if len(updated.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(updated.Paths))
	}
}

// TestUpdateLibrary_NotFound tests that updating non-existent library fails
func TestUpdateLibrary_NotFound(t *testing.T) {
	// Arrange - create temp directory for testing
	tempDir := t.TempDir()

	repo := newMockRepository()
	service := NewService(repo)
	nonExistentID := int64(999)

	// Act - use real path so we get past path validation to the NotFound error
	updated, err := service.UpdateLibrary(nonExistentID, "Name", []string{tempDir})

	// Assert
	if err != ErrLibraryNotFound {
		t.Errorf("Expected ErrLibraryNotFound, got %v", err)
	}

	if updated != nil {
		t.Error("Expected nil library, got non-nil")
	}
}

// TestUpdateLibrary_EmptyName tests that empty name is rejected
func TestUpdateLibrary_EmptyName(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)

	library := &Library{
		ID:    libraryID,
		Name:  "Music",
		Paths: []string{"/music"},
	}
	repo.libraries[1] = []*Library{library}

	// Act
	updated, err := service.UpdateLibrary(libraryID, "", []string{"/music"})

	// Assert
	if err != ErrNameRequired {
		t.Errorf("Expected ErrNameRequired, got %v", err)
	}

	if updated != nil {
		t.Error("Expected nil library, got non-nil")
	}
}

// TestUpdateLibrary_EmptyPaths tests that empty paths is rejected
func TestUpdateLibrary_EmptyPaths(t *testing.T) {
	// Arrange
	repo := newMockRepository()
	service := NewService(repo)
	libraryID := int64(1)

	library := &Library{
		ID:    libraryID,
		Name:  "Music",
		Paths: []string{"/music"},
	}
	repo.libraries[1] = []*Library{library}

	// Act
	updated, err := service.UpdateLibrary(libraryID, "Music", []string{})

	// Assert
	if err != ErrPathsRequired {
		t.Errorf("Expected ErrPathsRequired, got %v", err)
	}

	if updated != nil {
		t.Error("Expected nil library, got non-nil")
	}
}

func TestSearchLibrary(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo)

	libraryID := int64(1)
	repo.libraries[testUserID] = []*Library{{ID: libraryID, Name: "Test"}}
	repo.albums[1] = &Album{ID: 1, LibraryID: libraryID, Title: "Abbey Road"}
	repo.albums[2] = &Album{ID: 2, LibraryID: libraryID, Title: "Let It Be"}
	repo.artists[1] = &Artist{ID: 1, LibraryID: libraryID, Name: "The Beatles"}
	repo.artists[2] = &Artist{ID: 2, LibraryID: libraryID, Name: "Adele"}
	repo.tracks[1] = &Track{ID: 1, LibraryID: libraryID, Title: "Come Together"}

	t.Run("matches album title", func(t *testing.T) {
		result, err := service.Search(testUserID, libraryID, "abbey")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Albums) != 1 {
			t.Fatalf("want 1 album match, got %d", len(result.Albums))
		}
		if result.Albums[0].Album.Title != "Abbey Road" {
			t.Errorf("want \"Abbey Road\", got %q", result.Albums[0].Album.Title)
		}
	})

	t.Run("matches artist name", func(t *testing.T) {
		result, err := service.Search(testUserID, libraryID, "beatles")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Artists) != 1 {
			t.Fatalf("want 1 artist match, got %d", len(result.Artists))
		}
	})

	t.Run("matches track title", func(t *testing.T) {
		result, err := service.Search(testUserID, libraryID, "together")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Tracks) != 1 {
			t.Fatalf("want 1 track match, got %d", len(result.Tracks))
		}
	})

	t.Run("empty query returns empty result", func(t *testing.T) {
		result, err := service.Search(testUserID, libraryID, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Albums)+len(result.Artists)+len(result.Tracks) != 0 {
			t.Errorf("want empty result, got %+v", result)
		}
	})

	t.Run("no matches", func(t *testing.T) {
		result, err := service.Search(testUserID, libraryID, "xyzzy")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result.Albums)+len(result.Artists)+len(result.Tracks) != 0 {
			t.Errorf("want no matches, got %+v", result)
		}
	})
}

func TestListAlbumsHiResFilter(t *testing.T) {
	repo := newMockRepository()
	service := NewService(repo)

	libraryID := int64(1)
	repo.libraries[testUserID] = []*Library{{ID: libraryID, Name: "Test"}}
	repo.albums[1] = &Album{ID: 1, LibraryID: libraryID, Title: "Hi-Res Album", IsHiRes: true}
	repo.albums[2] = &Album{ID: 2, LibraryID: libraryID, Title: "Standard Album", IsHiRes: false}

	t.Run("without filter returns all albums", func(t *testing.T) {
		all, err := service.ListAlbums(testUserID, libraryID, ListAlbumsOptions{})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(all) != 2 {
			t.Errorf("want 2 albums, got %d", len(all))
		}
	})

	t.Run("HiResOnly returns only hi-res albums", func(t *testing.T) {
		hiRes, err := service.ListAlbums(testUserID, libraryID, ListAlbumsOptions{HiResOnly: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(hiRes) != 1 {
			t.Fatalf("want 1 hi-res album, got %d", len(hiRes))
		}
		if hiRes[0].Album.Title != "Hi-Res Album" {
			t.Errorf("want \"Hi-Res Album\", got %q", hiRes[0].Album.Title)
		}
	})
}
