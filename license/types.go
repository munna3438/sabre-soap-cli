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
	Payload     string `json:"pay,omitempty"`
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

func (l *License) ExpiresString() string {
	if l == nil {
		return "N/A"
	}
	return time.Unix(l.ExpiresAt, 0).UTC().Format("2006-01-02 15:04:05 MST")
}
