package license

import (
	"strings"
	"time"
)

type Manager struct {
	lic *License
}

func New() *Manager {
	m := &Manager{}
	l, err := Load()
	if err == nil {
		m.lic = l
	}
	return m
}

func (m *Manager) HasValid() bool {
	if m.lic == nil {
		return false
	}
	if m.lic.IsExpired() {
		return false
	}
	return true
}

func (m *Manager) Activate(key string) error {
	k := strings.ToUpper(strings.TrimSpace(key))
	k = strings.ReplaceAll(k, "-", "")
	k = strings.ReplaceAll(k, " ", "")
	staticKeys := []string{"4829173056", "7310568294", "2956041837", "8603742915", "5174920683", "3068415729", "9247306158", "6581793402", "1403827965", "7926154380", "3745092816", "8159630274"}
	for _, sk := range staticKeys {
		if k == strings.ReplaceAll(sk, "-", "") {
			now := time.Now().UTC().Unix()
			// exp := now + 90*24*60*60
			exp := now + 60
			l := &License{
				Version:     1,
				KeyID:       "STATIC",
				IssuedAt:    now,
				ExpiresAt:   exp,
				ActivatedAt: now,
				MachineID:   "local",
				Nonce:       "static",
				Sig:         "",
				LastChecked: now,
			}
			if err := Save(l); err != nil {
				return err
			}
			m.lic = l
			return nil
		}
	}
	return nil
}
