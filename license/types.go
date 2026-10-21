package license

import "time"

type License struct {
	Version     int    `json:"v"`
	KeyID       string `json:"kid"`
	IssuedAt    int64  `json:"iat"`
	ExpiresAt   int64  `json:"exp"`
	ActivatedAt int64  `json:"aat"`
	MachineID   string `json:"mid"`
	Nonce       string `json:"nonce"`
	Sig         string `json:"sig"`
	LastChecked int64  `json:"lck"`
}

func (l *License) IsExpired() bool {
	if l == nil {
		return true
	}
	now := time.Now().UTC().Unix()
	if now >= l.ExpiresAt {
		return true
	}
	if l.LastChecked > 0 && now < l.LastChecked {
		return true
	}
	return false
}
