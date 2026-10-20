package license

import (
	"crypto/ed25519"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
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
	if m.lic.MachineID != GetMachineID() {
		return false
	}
	if !verifyLicense(m.lic) {
		return false
	}
	return true
}

func (m *Manager) Get() *License {
	return m.lic
}

func (m *Manager) ExpiresAt() string {
	if m.lic == nil {
		return ""
	}
	return m.lic.ExpiresString()
}

func (m *Manager) MarkChecked() {
	if m.lic != nil {
		m.lic.LastChecked = time.Now().UTC().Unix()
		_ = Save(m.lic)
	}
}

func (m *Manager) Activate(key string) error {
	key = strings.ToUpper(strings.TrimSpace(key))
	key = strings.ReplaceAll(key, "-", "")
	key = strings.ReplaceAll(key, " ", "")

	blob, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(key)
	if err != nil {
		return errors.New("invalid key format")
	}
	if len(blob) < 2+ed25519.SignatureSize {
		return errors.New("invalid key format")
	}

	pLen := int(binary.BigEndian.Uint16(blob[0:2]))
	if pLen <= 0 || 2+pLen+ed25519.SignatureSize != len(blob) {
		return errors.New("invalid key format")
	}

	payload := blob[2 : 2+pLen]
	sigBytes := blob[2+pLen : 2+pLen+ed25519.SignatureSize]

	type p struct {
		V     int    `json:"v"`
		KeyID string `json:"kid"`
		IAT   int64  `json:"iat"`
		EXP   int64  `json:"exp"`
		Nonce string `json:"nonce"`
	}
	var pp p
	if err := json.Unmarshal(payload, &pp); err != nil {
		return errors.New("invalid key")
	}

	pub, err := getPubKey()
	if err != nil {
		return err
	}

	if !ed25519.Verify(pub, payload, sigBytes) {
		return errors.New("invalid key signature")
	}

	mid := GetMachineID()
	now := time.Now().UTC().Unix()
	exp := now + 90*24*60*60

	l := &License{
		Version:     1,
		KeyID:       pp.KeyID,
		IssuedAt:    pp.IAT,
		ExpiresAt:   exp,
		ActivatedAt: now,
		MachineID:   mid,
		Nonce:       pp.Nonce,
		Sig:         base64.RawURLEncoding.EncodeToString(sigBytes),
		LastChecked: now,
	}

	l.Payload = string(payload)
	l.Sig = base64.RawURLEncoding.EncodeToString(sigBytes)
	if err := Save(l); err != nil {
		return err
	}
	m.lic = l
	return nil
}
