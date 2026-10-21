package license

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

func nowUnix() int64 {
	return time.Now().UTC().Unix()
}

func licensePath() (string, error) {
	base := os.Getenv("LOCALAPPDATA")
	if base == "" {
		base = os.Getenv("APPDATA")
	}
	if base == "" {
		return "", os.ErrNotExist
	}
	dir := filepath.Join(base, "SabreMonitor")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "license.dat"), nil
}

func Save(l *License) error {
	l.LastChecked = nowUnix()
	b, err := json.Marshal(l)
	if err != nil {
		return err
	}
	p, err := licensePath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

func Load() (*License, error) {
	p, err := licensePath()
	if err != nil {
		return nil, nil
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, nil
	}
	var l License
	if err := json.Unmarshal(raw, &l); err != nil {
		return nil, nil
	}
	l.LastChecked = nowUnix()
	_ = Save(&l)
	return &l, nil
}
