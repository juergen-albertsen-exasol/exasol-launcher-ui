package imports_test

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/exasol/exasol-launcher-ui/internal/imports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const deployment = "default"

type clock struct {
	mutex sync.Mutex
	now   time.Time
}

func (c *clock) Now() time.Time {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	return c.now
}

func (c *clock) Advance(duration time.Duration) {
	c.mutex.Lock()
	defer c.mutex.Unlock()

	c.now = c.now.Add(duration)
}

func newUploads(t *testing.T) (*imports.Uploads, *clock, string) {
	t.Helper()

	dir := filepath.Join(t.TempDir(), "uploads")
	uploads := imports.NewUploads(dir)
	fake := &clock{now: time.Date(2026, 11, 13, 9, 0, 0, 0, time.UTC)}
	uploads.SetClock(fake.Now)
	t.Cleanup(func() { _ = uploads.Close() })

	return uploads, fake, dir
}

func TestAnUploadStoresItsFilesUnderServerNames(t *testing.T) {
	t.Parallel()

	uploads, _, dir := newUploads(t)

	created, err := uploads.Create(deployment, []imports.FileSpec{
		{Name: "../trips 2024 – Jänner.parquet", Size: 3},
		{Name: "b.parquet", Size: 0},
	})
	require.NoError(t, err)

	require.NoError(t, uploads.Store(deployment, created.ID, 0, strings.NewReader("abc")))

	_, err = uploads.Get(deployment, created.ID)
	require.ErrorIs(t, err, imports.ErrIncomplete)

	require.NoError(t, uploads.Store(deployment, created.ID, 1, strings.NewReader("")))

	complete, err := uploads.Get(deployment, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created, complete)
	assert.Equal(t, filepath.Join(dir, created.ID, "0.parquet"), uploads.Path(created.ID, 0))

	content, err := os.ReadFile(uploads.Path(created.ID, 0))
	require.NoError(t, err)
	assert.Equal(t, "abc", string(content))

	info, err := os.Stat(filepath.Join(dir, created.ID))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o700), info.Mode().Perm())
}

func TestAnUploadHoldsOneToAHundredFiles(t *testing.T) {
	t.Parallel()

	uploads, _, _ := newUploads(t)

	_, none := uploads.Create(deployment, nil)
	_, tooMany := uploads.Create(deployment, make([]imports.FileSpec, imports.MaxFiles+1))
	_, unnamed := uploads.Create(deployment, []imports.FileSpec{{Size: 1}})

	require.Error(t, none)
	require.Error(t, tooMany)
	require.Error(t, unnamed)
}

func TestAFileOfAnotherSizeIsRefused(t *testing.T) {
	t.Parallel()

	uploads, _, _ := newUploads(t)
	created, err := uploads.Create(deployment, []imports.FileSpec{{Name: "a.parquet", Size: 3}})
	require.NoError(t, err)

	short := uploads.Store(deployment, created.ID, 0, strings.NewReader("ab"))
	long := uploads.Store(deployment, created.ID, 0, strings.NewReader("abcd"))

	require.ErrorIs(t, short, imports.ErrSizeDiffers)
	require.ErrorIs(t, long, imports.ErrSizeDiffers)
	assert.NoFileExists(t, uploads.Path(created.ID, 0))
}

func TestAnUploadBelongsToItsDeployment(t *testing.T) {
	t.Parallel()

	uploads, _, _ := newUploads(t)
	created, err := uploads.Create(deployment, []imports.FileSpec{{Name: "a.parquet", Size: 1}})
	require.NoError(t, err)

	otherDeployment := uploads.Store("test", created.ID, 0, strings.NewReader("a"))
	unknownFile := uploads.Store(deployment, created.ID, 1, strings.NewReader("a"))
	_, unknownUpload := uploads.Get(deployment, "unknown")

	require.ErrorIs(t, otherDeployment, imports.ErrNoUpload)
	require.ErrorIs(t, unknownFile, imports.ErrNoFile)
	require.ErrorIs(t, unknownUpload, imports.ErrNoUpload)
}

func storedUpload(t *testing.T, uploads *imports.Uploads) imports.Upload {
	t.Helper()

	created, err := uploads.Create(deployment, []imports.FileSpec{{Name: "a.parquet", Size: 1}})
	require.NoError(t, err)
	require.NoError(t, uploads.Store(deployment, created.ID, 0, strings.NewReader("a")))

	return created
}

func TestUploadsExpireADayAfterTheirLastUse(t *testing.T) {
	t.Parallel()

	uploads, fake, dir := newUploads(t)
	created := storedUpload(t, uploads)

	fake.Advance(23 * time.Hour)
	uploads.RemoveExpired()

	_, err := uploads.Get(deployment, created.ID)
	require.NoError(t, err)

	fake.Advance(24*time.Hour + time.Second)
	uploads.RemoveExpired()

	_, err = uploads.Get(deployment, created.ID)
	require.ErrorIs(t, err, imports.ErrNoUpload)
	assert.NoDirExists(t, filepath.Join(dir, created.ID))
}

func TestAnUploadInUseDoesNotExpire(t *testing.T) {
	t.Parallel()

	uploads, fake, _ := newUploads(t)
	created := storedUpload(t, uploads)

	_, err := uploads.Acquire(deployment, created.ID)
	require.NoError(t, err)

	fake.Advance(48 * time.Hour)
	uploads.RemoveExpired()

	_, err = uploads.Get(deployment, created.ID)
	require.NoError(t, err)
}

func TestReleasingAnUploadCanDeleteIt(t *testing.T) {
	t.Parallel()

	uploads, _, dir := newUploads(t)
	created := storedUpload(t, uploads)

	_, err := uploads.Acquire(deployment, created.ID)
	require.NoError(t, err)
	uploads.Release(created.ID, true)

	_, err = uploads.Get(deployment, created.ID)
	require.ErrorIs(t, err, imports.ErrNoUpload)
	assert.NoDirExists(t, filepath.Join(dir, created.ID))
}

func TestClosingDeletesTheUploads(t *testing.T) {
	t.Parallel()

	uploads, _, dir := newUploads(t)
	storedUpload(t, uploads)

	require.NoError(t, uploads.Close())

	assert.NoDirExists(t, dir)
}

func TestClosingKeepsAnotherServersUploads(t *testing.T) {
	t.Parallel()

	uploads, _, dir := newUploads(t)
	storedUpload(t, uploads)

	other := imports.NewUploads(dir)
	otherUpload := storedUpload(t, other)

	require.NoError(t, uploads.Close())

	assert.FileExists(t, other.Path(otherUpload.ID, 0))
	require.NoError(t, other.Close())
}
