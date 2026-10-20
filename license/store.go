package license

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

func nowUnix() int64 {
	return time.Now().UTC().Unix()
}

func licensePath() (string, error) {
	var base string
	if runtime.GOOS == "windows" {
		base = os.Getenv("LOCALAPPDATA")
		if base == "" {
			base = os.Getenv("APPDATA")
		}
	} else {
		home, _ := os.UserHomeDir()
		base = filepath.Join(home, ".config")
	}
	if base == "" {
		return "", errors.New("license: cannot determine appdata")
	}
	dir := filepath.Join(base, "SabreMonitor")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	return filepath.Join(dir, "license.dat"), nil
}

func Save(l *License) error {
	mid := GetMachineID()
	l.MachineID = mid
	l.LastChecked = nowUnix()
	data, err := encryptJSON(l, mid)
	if err != nil {
		return err
	}
	p, err := licensePath()
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}

func Load() (*License, error) {
	p, err := licensePath()
	if err != nil {
		return nil, err
	}
	raw, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	mid := GetMachineID()
	var l License
	if err := decryptJSON(raw, mid, &l); err != nil {
		return nil, err
	}
	l.LastChecked = nowUnix()
	_ = Save(&l)
	return &l, nil
}
