package httpcache

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Entry is the on-disk representation of a cached HTTP response.
// ETag is not stored separately — it lives in Header and is read
// via Header.Get("ETag") at the call site, keeping a single source
// of truth.
type Entry struct {
	StatusCode   int         `json:"status_code"`
	Header       http.Header `json:"header"`
	LastModified time.Time   `json:"last_modified,omitzero"`
	CachedAt     time.Time   `json:"cached_at"`
}

// DiskStore reads and writes cache entries to a directory on disk.
// Each entry is stored as two files: <prefix>/<key>.meta.json for
// metadata and <prefix>/<key>.body for the raw response body.
// The key's first two hex characters form a subdirectory to avoid
// overcrowding a single folder.
type DiskStore struct {
	Dir string
}

// NewDiskStore returns a store rooted at dir. The directory tree is
// created lazily on the first Put.
func NewDiskStore(dir string) *DiskStore {
	return &DiskStore{Dir: dir}
}

func (s *DiskStore) entryDir(key string) string {
	return filepath.Join(s.Dir, key[:2])
}

func (s *DiskStore) metaPath(key string) string {
	return filepath.Join(s.entryDir(key), key+".meta.json")
}

func (s *DiskStore) bodyPath(key string) string {
	return filepath.Join(s.entryDir(key), key+".body")
}

// Get loads a cache entry from disk. Returns the metadata and raw
// response body. Any I/O or decode error means a cache miss.
func (s *DiskStore) Get(key string) (*Entry, []byte, error) {
	metaData, err := os.ReadFile(s.metaPath(key))
	if err != nil {
		return nil, nil, err
	}

	var entry Entry
	if err := json.Unmarshal(metaData, &entry); err != nil {
		return nil, nil, err
	}

	body, err := os.ReadFile(s.bodyPath(key))
	if err != nil {
		return nil, nil, err
	}

	return &entry, body, nil
}

// PutMeta writes only the metadata file, leaving the body file
// unchanged. Used when only CachedAt needs refreshing (e.g. 304
// revalidation) to avoid rewriting the unchanged body.
func (s *DiskStore) PutMeta(key string, entry *Entry) error {
	dir := s.entryDir(key)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	metaData, err := json.MarshalIndent(entry, "", "  ")
	if err != nil {
		return err
	}

	return atomicWrite(s.metaPath(key), metaData)
}

// Put writes a cache entry to disk. Writes are performed via a
// temporary file + rename so a crash mid-write never leaves a
// corrupt entry. Body is written first so a crash between the two
// writes leaves an orphan body (harmless) rather than a meta
// pointing at a missing body (broken).
func (s *DiskStore) Put(key string, entry *Entry, body []byte) error {
	if err := os.MkdirAll(s.entryDir(key), 0o755); err != nil {
		return err
	}
	if err := atomicWrite(s.bodyPath(key), body); err != nil {
		return err
	}
	return s.PutMeta(key, entry)
}

// atomicWrite writes data to path via a temp file + rename so a
// crash never leaves a half-written file at the target path.
func atomicWrite(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
