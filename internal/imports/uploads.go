package imports

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const (
	// MaxFiles is the number of files one upload may hold.
	MaxFiles = 100
	// uploadLifetime is how long an upload is kept after its last use.
	uploadLifetime = 24 * time.Hour
	janitorPeriod  = 10 * time.Minute
	randomIDBytes  = 16
	directoryMode  = 0o700
	fileMode       = 0o600
)

var (
	ErrNoUpload    = errors.New("the upload does not exist or was deleted")
	ErrNoFile      = errors.New("the upload has no such file")
	ErrIncomplete  = errors.New("not every file of the upload has arrived")
	ErrSizeDiffers = errors.New("the file's size differs from the size announced")
)

// FileSpec names a file of an upload and its size in bytes.
type FileSpec struct {
	Name string
	Size int64
}

// Upload is a set of files the browser sent for an import.
type Upload struct {
	ID         string
	Deployment string
	Files      []FileSpec
}

// Uploads keeps uploaded files in a folder on the host, named by the server
// so that user file names never reach the driver's path check.
type Uploads struct {
	dir string
	now func() time.Time

	mutex   sync.Mutex
	uploads map[string]*upload
	closed  bool

	janitor     sync.Once
	stopJanitor chan struct{}
}

type upload struct {
	Upload

	stored   []bool
	lastUsed time.Time
	// inUse counts the imports reading the upload, which keep it.
	inUse int
}

// DefaultUploadsDir is the folder uploads are kept in by default.
func DefaultUploadsDir() string {
	return filepath.Join(os.TempDir(), "exasol-local-ui", "uploads")
}

// NewUploads keeps uploads in dir, which the first upload creates.
func NewUploads(dir string) *Uploads {
	return &Uploads{
		dir:         dir,
		now:         time.Now,
		uploads:     map[string]*upload{},
		stopJanitor: make(chan struct{}),
	}
}

// SetClock replaces the clock that decides when uploads expire.
func (u *Uploads) SetClock(now func() time.Time) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	u.now = now
}

// Create starts an upload of files for deployment.
func (u *Uploads) Create(deployment string, files []FileSpec) (Upload, error) {
	if len(files) == 0 || len(files) > MaxFiles {
		return Upload{}, fmt.Errorf("an upload holds 1 to %d files", MaxFiles)
	}

	for _, file := range files {
		if file.Name == "" || file.Size < 0 {
			return Upload{}, errors.New("every file needs a name and a size")
		}
	}

	u.janitor.Do(func() { go u.removeExpiredPeriodically() })

	created := &upload{
		ID:         newRandomID(),
		Deployment: deployment,
		Files:      append([]FileSpec(nil), files...),
		stored:     make([]bool, len(files)),
	}

	if err := os.MkdirAll(filepath.Join(u.dir, created.ID), directoryMode); err != nil {
		return Upload{}, fmt.Errorf("cannot create the upload's folder: %w", err)
	}

	u.mutex.Lock()
	defer u.mutex.Unlock()

	if u.closed {
		return Upload{}, ErrNoUpload
	}

	created.lastUsed = u.now()
	u.uploads[created.ID] = created

	return created.Upload, nil
}

// Store writes the file at index of the upload from body, which must
// hold exactly the file's declared size.
func (u *Uploads) Store(deployment, uploadID string, index int, body io.Reader) error {
	file, err := u.file(deployment, uploadID, index)
	if err != nil {
		return err
	}

	path := u.Path(uploadID, index)

	output, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, fileMode)
	if err != nil {
		return fmt.Errorf("cannot store %s: %w", file.Name, err)
	}

	written, err := io.Copy(output, io.LimitReader(body, file.Size+1))

	err = errors.Join(err, output.Close())
	if err == nil && written != file.Size {
		err = fmt.Errorf("%w: %s has %d bytes rather than %d",
			ErrSizeDiffers, file.Name, written, file.Size)
	}

	if err != nil {
		_ = os.Remove(path)

		return err
	}

	u.mutex.Lock()
	defer u.mutex.Unlock()

	if found, ok := u.uploads[uploadID]; ok {
		found.stored[index] = true
		found.lastUsed = u.now()
	}

	return nil
}

// Get returns the upload of deployment once all its files arrived.
func (u *Uploads) Get(deployment, uploadID string) (Upload, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	found, ok := u.uploads[uploadID]
	if !ok || found.Deployment != deployment {
		return Upload{}, ErrNoUpload
	}

	for _, stored := range found.stored {
		if !stored {
			return Upload{}, ErrIncomplete
		}
	}

	found.lastUsed = u.now()

	return found.Upload, nil
}

// Path returns where the file at index of the upload is stored.
func (u *Uploads) Path(uploadID string, index int) string {
	return filepath.Join(u.dir, uploadID, strconv.Itoa(index)+".parquet")
}

// Acquire keeps the upload while an import reads it; Release ends that.
func (u *Uploads) Acquire(deployment, uploadID string) (Upload, error) {
	found, err := u.Get(deployment, uploadID)
	if err != nil {
		return Upload{}, err
	}

	u.mutex.Lock()
	defer u.mutex.Unlock()

	if held, ok := u.uploads[uploadID]; ok {
		held.inUse++
	}

	return found, nil
}

// Release ends an Acquire, and deletes the upload when remove is set.
func (u *Uploads) Release(uploadID string, remove bool) {
	u.mutex.Lock()

	found, ok := u.uploads[uploadID]
	if !ok {
		u.mutex.Unlock()

		return
	}

	found.inUse--
	found.lastUsed = u.now()

	if remove {
		delete(u.uploads, uploadID)
	}

	u.mutex.Unlock()

	if remove {
		_ = os.RemoveAll(filepath.Join(u.dir, uploadID))
	}
}

// RemoveExpired deletes the uploads unused for 24 hours.
func (u *Uploads) RemoveExpired() {
	u.mutex.Lock()

	deadline := u.now().Add(-uploadLifetime)

	var expired []string

	for uploadID, found := range u.uploads {
		if found.inUse == 0 && found.lastUsed.Before(deadline) {
			expired = append(expired, uploadID)
			delete(u.uploads, uploadID)
		}
	}

	u.mutex.Unlock()

	for _, uploadID := range expired {
		_ = os.RemoveAll(filepath.Join(u.dir, uploadID))
	}
}

// Close deletes every upload and, when no other server keeps uploads in
// it, the uploads folder.
func (u *Uploads) Close() error {
	u.mutex.Lock()

	if u.closed {
		u.mutex.Unlock()

		return nil
	}

	u.closed = true
	uploads := u.uploads
	u.uploads = map[string]*upload{}
	close(u.stopJanitor)
	u.mutex.Unlock()

	errs := make([]error, 0, len(uploads))
	for uploadID := range uploads {
		errs = append(errs, os.RemoveAll(filepath.Join(u.dir, uploadID)))
	}

	// Another server's uploads keep the folder, which then is not empty.
	_ = os.Remove(u.dir)

	return errors.Join(errs...)
}

func (u *Uploads) file(deployment, uploadID string, index int) (FileSpec, error) {
	u.mutex.Lock()
	defer u.mutex.Unlock()

	found, ok := u.uploads[uploadID]
	if !ok || found.Deployment != deployment {
		return FileSpec{}, ErrNoUpload
	}

	if index < 0 || index >= len(found.Files) {
		return FileSpec{}, ErrNoFile
	}

	found.lastUsed = u.now()

	return found.Files[index], nil
}

func (u *Uploads) removeExpiredPeriodically() {
	ticker := time.NewTicker(janitorPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-u.stopJanitor:
			return
		case <-ticker.C:
			u.RemoveExpired()
		}
	}
}

func newRandomID() string {
	uploadID := make([]byte, randomIDBytes)
	_, _ = rand.Read(uploadID)

	return hex.EncodeToString(uploadID)
}
