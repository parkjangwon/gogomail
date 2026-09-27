package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
)

// Bundle represents an on-disk backup bundle directory. It accumulates files
// (recording their checksums) and writes a manifest describing them.
type Bundle struct {
	dir      string
	manifest Manifest
}

// NewBundle prepares a bundle rooted at dir, which must not already contain a
// manifest. The directory is created if missing.
func NewBundle(dir string, manifest Manifest) (*Bundle, error) {
	if dir == "" {
		return nil, fmt.Errorf("bundle directory is required")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, fmt.Errorf("create bundle directory: %w", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ManifestFileName)); err == nil {
		return nil, fmt.Errorf("bundle directory %q already contains %s", dir, ManifestFileName)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect bundle directory: %w", err)
	}
	return &Bundle{dir: dir, manifest: manifest}, nil
}

// Dir returns the bundle root directory.
func (b *Bundle) Dir() string { return b.dir }

// SetStorageObjectCount records the number of storage objects captured, which is
// only known after the storage snapshot step runs. It is written into the
// manifest by Finalize.
func (b *Bundle) SetStorageObjectCount(n int) { b.manifest.StorageObjectCount = n }

// AddFile writes content to name inside the bundle, records its size and
// SHA-256 checksum in the manifest, and returns the resulting FileEntry.
func (b *Bundle) AddFile(name string, content io.Reader) (FileEntry, error) {
	if name == "" {
		return FileEntry{}, fmt.Errorf("file name is required")
	}
	if name == ManifestFileName {
		return FileEntry{}, fmt.Errorf("%s is reserved and written by Finalize", ManifestFileName)
	}
	if filepath.Base(name) != name {
		return FileEntry{}, fmt.Errorf("bundle file name %q must not contain path separators", name)
	}
	dst := filepath.Join(b.dir, name)
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o640)
	if err != nil {
		return FileEntry{}, fmt.Errorf("create bundle file: %w", err)
	}
	h := sha256.New()
	size, copyErr := io.Copy(io.MultiWriter(f, h), content)
	closeErr := f.Close()
	if copyErr != nil {
		return FileEntry{}, fmt.Errorf("write bundle file: %w", copyErr)
	}
	if closeErr != nil {
		return FileEntry{}, fmt.Errorf("close bundle file: %w", closeErr)
	}
	entry := FileEntry{Name: name, Size: size, SHA256: hex.EncodeToString(h.Sum(nil))}
	b.manifest.Files = append(b.manifest.Files, entry)
	return entry, nil
}

// AddJSON marshals v as indented JSON and adds it to the bundle under name.
func (b *Bundle) AddJSON(name string, v any) (FileEntry, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return FileEntry{}, fmt.Errorf("marshal %s: %w", name, err)
	}
	return b.AddFile(name, bytesReader(data))
}

// Finalize writes manifest.json (with files sorted for determinism) and returns
// the completed manifest.
func (b *Bundle) Finalize() (Manifest, error) {
	sort.Slice(b.manifest.Files, func(i, j int) bool {
		return b.manifest.Files[i].Name < b.manifest.Files[j].Name
	})
	if b.manifest.ManifestVersion == 0 {
		b.manifest.ManifestVersion = ManifestVersion
	}
	data, err := json.MarshalIndent(b.manifest, "", "  ")
	if err != nil {
		return Manifest{}, fmt.Errorf("marshal manifest: %w", err)
	}
	dst := filepath.Join(b.dir, ManifestFileName)
	if err := os.WriteFile(dst, data, 0o640); err != nil {
		return Manifest{}, fmt.Errorf("write manifest: %w", err)
	}
	return b.manifest, nil
}

// LoadManifest reads and parses manifest.json from a bundle directory.
func LoadManifest(dir string) (Manifest, error) {
	data, err := os.ReadFile(filepath.Join(dir, ManifestFileName))
	if err != nil {
		return Manifest{}, fmt.Errorf("read manifest: %w", err)
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return Manifest{}, fmt.Errorf("parse manifest: %w", err)
	}
	return m, nil
}

// ValidateBundle verifies that a bundle directory is well-formed:
//   - the manifest schema version is understood,
//   - every file listed in the manifest exists with a matching size and SHA-256,
//   - the required database file is present.
//
// It returns a descriptive error on the first problem found.
func ValidateBundle(dir string) (Manifest, error) {
	m, err := LoadManifest(dir)
	if err != nil {
		return Manifest{}, err
	}
	if m.ManifestVersion != ManifestVersion {
		return m, fmt.Errorf("unsupported manifest version %d (this build understands %d)", m.ManifestVersion, ManifestVersion)
	}
	if _, ok := m.FindFile(DatabaseFileName); !ok {
		return m, fmt.Errorf("manifest is missing required database file %q", DatabaseFileName)
	}
	for _, entry := range m.Files {
		path := filepath.Join(dir, entry.Name)
		info, err := os.Stat(path)
		if err != nil {
			return m, fmt.Errorf("bundle file %q missing: %w", entry.Name, err)
		}
		if info.Size() != entry.Size {
			return m, fmt.Errorf("bundle file %q size mismatch: manifest %d, on disk %d", entry.Name, entry.Size, info.Size())
		}
		sum, err := hashFile(path)
		if err != nil {
			return m, fmt.Errorf("checksum bundle file %q: %w", entry.Name, err)
		}
		if sum != entry.SHA256 {
			return m, fmt.Errorf("bundle file %q checksum mismatch: manifest %s, computed %s", entry.Name, entry.SHA256, sum)
		}
	}
	return m, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
