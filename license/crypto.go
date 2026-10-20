package license

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// PASTE YOUR ED25519 PUBLIC KEY (32 bytes) BASE64 HERE
const publicKeyB64 = "Fwj1MD2/ckdn6iWgaeVTKEsFV0UwZQv1dRH0XF2ZHqE="

func sha256Bytes(b []byte) []byte {
	h := sha256.Sum256(b)
	return h[:]
}

func deriveFileKey(mid string) []byte {
	seed := sha256Bytes([]byte("sabre-monitor-v1|" + mid))
	k := sha256Bytes(seed)
	return k
}

func getPubKey() (ed25519.PublicKey, error) {
	if publicKeyB64 == "" || strings.Contains(publicKeyB64, "PASTE") {
		return nil, errors.New("license: public key not embedded")
	}
	raw, err := base64.StdEncoding.DecodeString(publicKeyB64)
	if err != nil {
		raw, err = base64.RawStdEncoding.DecodeString(publicKeyB64)
		if err != nil {
			return nil, err
		}
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("license: invalid public key length")
	}
	return ed25519.PublicKey(raw), nil
}

func decodeSig(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return nil, errors.New("bad sig")
}

func verifyLicense(l *License) bool {
	if l == nil {
		return false
	}
	pub, err := getPubKey()
	if err != nil {
		return false
	}
	msg := fmt.Sprintf("%s|%d|%d|%s|%s", l.KeyID, l.IssuedAt, l.ExpiresAt, l.Nonce, l.MachineID)
	sig, err := decodeSig(l.Sig)
	if err != nil {
		return false
	}
	if len(sig) != ed25519.SignatureSize {
		return false
	}
	return ed25519.Verify(pub, []byte(msg), sig)
}

func encryptJSON(v any, mid string) ([]byte, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	key := deriveFileKey(mid)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nonce, nonce, b, nil)
	return ct, nil
}

func decryptJSON(data []byte, mid string, out any) error {
	key := deriveFileKey(mid)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}
	if len(data) < gcm.NonceSize() {
		return errors.New("license: bad file")
	}
	nonce := data[:gcm.NonceSize()]
	ct := data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return err
	}
	return json.Unmarshal(plain, out)
}
