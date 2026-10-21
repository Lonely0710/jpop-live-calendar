package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"jpop-live-calendar/internal/domain"
)

// Version is the layout of the cache file. Bump it when the saved shape
// or the meaning of the announcements changes; older files are then
// ignored rather than misread.
const Version = 1

type file struct {
	Version       int                         `json:"version"`
	SavedAt       time.Time                   `json:"saved_at"`
	AppVersion    string                      `json:"app_version,omitempty"`
	Announcements []domain.PublicAnnouncement `json:"announcements"`
}

// Load reads the announcements of path. A missing file is not an error.
// A corrupt file is renamed to "<path>.corrupt" (best effort) and
// reported, so the app can start empty and write a new snapshot.
func Load(path string) ([]domain.PublicAnnouncement, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取缓存 %s：%w", path, err)
	}
	var f file
	if err := json.Unmarshal(b, &f); err != nil {
		moved := path + ".corrupt"
		if rerr := os.Rename(path, moved); rerr != nil {
			return nil, fmt.Errorf("缓存文件损坏（未能移到 %s：%v）：%w", moved, rerr, err)
		}
		return nil, fmt.Errorf("缓存文件损坏（已移到 %s）：%w", moved, err)
	}
	if f.Version != Version {
		return nil, fmt.Errorf("缓存版本为 %d，当前只支持 %d，已忽略", f.Version, Version)
	}
	return f.Announcements, nil
}

// Save writes the announcements to path atomically. An empty list keeps
// the previous snapshot: nothing new was learned, and a cache that failed
// to load must not be replaced by an empty one.
func Save(path, appVersion string, anns []domain.PublicAnnouncement) error {
	if len(anns) == 0 {
		return nil
	}
	b, err := json.Marshal(file{
		Version:       Version,
		SavedAt:       time.Now(),
		AppVersion:    appVersion,
		Announcements: anns,
	})
	if err != nil {
		return fmt.Errorf("编码缓存：%w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建缓存目录 %s：%w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".cache-*.tmp")
	if err != nil {
		return fmt.Errorf("创建缓存临时文件：%w", err)
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(name)
		return fmt.Errorf("写入缓存：%w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return fmt.Errorf("写入缓存：%w", err)
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return fmt.Errorf("替换缓存 %s：%w", path, err)
	}
	return nil
}
