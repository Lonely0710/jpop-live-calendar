package cache

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"jpop-live-calendar/internal/avatar"
)

// AvatarVersion is the layout of the avatar cache. Bump it when the saved
// shape changes; older files are ignored rather than misread.
const AvatarVersion = 2 // 2: the Deezer lookup picks the best-known exact match

type avatarFile struct {
	Version int                     `json:"version"`
	SavedAt time.Time               `json:"saved_at"`
	Avatars map[string]cachedAvatar `json:"avatars"`
}

// cachedAvatar is one public picture and what is needed to revalidate it.
type cachedAvatar struct {
	ArtistID     string     `json:"artist_id"`
	URL          string     `json:"url"`
	Source       string     `json:"source"`
	Image        []byte     `json:"image_png"`
	Color        color.RGBA `json:"color"`
	Share        float64    `json:"share"`
	Manual       bool       `json:"manual"`
	ETag         string     `json:"etag,omitempty"`
	LastModified string     `json:"last_modified,omitempty"`
	Hash         string     `json:"hash,omitempty"`
	FetchedAt    time.Time  `json:"fetched_at"`
}

// LoadAvatars reads the public pictures saved by SaveCache. A missing file
// is not an error; a corrupt one is renamed to "<path>.corrupt".
func LoadAvatars(path string) (map[string]avatar.Avatar, error) {
	b, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]avatar.Avatar{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("读取头像缓存 %s：%w", path, err)
	}
	var f avatarFile
	if err := json.Unmarshal(b, &f); err != nil {
		moved := path + ".corrupt"
		if rerr := os.Rename(path, moved); rerr != nil {
			return nil, fmt.Errorf("头像缓存损坏（未能移到 %s：%v）：%w", moved, rerr, err)
		}
		return nil, fmt.Errorf("头像缓存损坏（已移到 %s）：%w", moved, err)
	}
	if f.Version != AvatarVersion {
		return nil, fmt.Errorf("头像缓存版本为 %d，当前只支持 %d，已忽略", f.Version, AvatarVersion)
	}
	out := make(map[string]avatar.Avatar, len(f.Avatars))
	for id, c := range f.Avatars {
		if len(c.Image) == 0 {
			continue
		}
		img, err := png.Decode(bytes.NewReader(c.Image))
		if err != nil {
			continue // one unreadable picture must not throw the rest away
		}
		out[id] = avatar.Avatar{
			ArtistID: c.ArtistID, URL: c.URL, Source: c.Source, Image: img,
			Color: c.Color, Share: c.Share, Manual: c.Manual,
			ETag: c.ETag, LastModified: c.LastModified, Hash: c.Hash, FetchedAt: c.FetchedAt,
		}
	}
	return out, nil
}

// SaveAvatars writes the public pictures to path atomically. Only pictures
// are saved, never user data or error text.
func SaveAvatars(path string, avatars map[string]avatar.Avatar) error {
	f := avatarFile{
		Version: AvatarVersion,
		SavedAt: time.Now(),
		Avatars: map[string]cachedAvatar{},
	}
	for id, a := range avatars {
		if a.Image == nil {
			continue
		}
		var buf bytes.Buffer
		if err := png.Encode(&buf, a.Image); err != nil {
			continue
		}
		f.Avatars[id] = cachedAvatar{
			ArtistID: a.ArtistID, URL: a.URL, Source: a.Source, Image: buf.Bytes(),
			Color: a.Color, Share: a.Share, Manual: a.Manual,
			ETag: a.ETag, LastModified: a.LastModified, Hash: a.Hash, FetchedAt: a.FetchedAt,
		}
	}
	b, err := json.Marshal(f)
	if err != nil {
		return fmt.Errorf("编码头像缓存：%w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建头像缓存目录 %s：%w", dir, err)
	}
	tmp, err := os.CreateTemp(dir, ".avatars-*.tmp")
	if err != nil {
		return fmt.Errorf("创建头像缓存临时文件：%w", err)
	}
	name := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(name)
		return fmt.Errorf("写入头像缓存：%w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return fmt.Errorf("写入头像缓存：%w", err)
	}
	if err := os.Rename(name, path); err != nil {
		os.Remove(name)
		return fmt.Errorf("替换头像缓存 %s：%w", path, err)
	}
	return nil
}
