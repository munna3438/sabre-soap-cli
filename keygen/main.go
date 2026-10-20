package main

import (
	"bufio"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

func main() {
	reader := bufio.NewScanner(os.Stdin)
	reader.Buffer(make([]byte, 1024*1024), 1024*1024)
	reader.Split(bufio.ScanLines)

	fmt.Println("=== SABRE MONITOR KEYGEN (OFFLINE) ===")
	fmt.Println("This uses your ED25519 PRIVATE KEY. DO NOT SHARE IT.")
	fmt.Print("Enter private key file path (ed25519 priv key, 64 bytes raw or base64): ")
	if !reader.Scan() {
		return
	}
	privPath := strings.TrimSpace(reader.Text())

	privRaw, err := os.ReadFile(privPath)
	if err != nil {
		panic(err)
	}

	var priv ed25519.PrivateKey
	if len(privRaw) == ed25519.PrivateKeySize {
		priv = ed25519.PrivateKey(privRaw)
	} else {
		s := strings.TrimSpace(string(privRaw))
		if b, err2 := base64.StdEncoding.DecodeString(s); err2 == nil && len(b) == ed25519.PrivateKeySize {
			priv = ed25519.PrivateKey(b)
		} else if b, err2 := base64.RawStdEncoding.DecodeString(s); err2 == nil && len(b) == ed25519.PrivateKeySize {
			priv = ed25519.PrivateKey(b)
		} else {
			panic("invalid private key")
		}
	}

	fmt.Print("Client KeyID (e.g. CLIENT-ABC123): ")
	if !reader.Scan() {
		return
	}
	kid := strings.TrimSpace(reader.Text())
	if kid == "" {
		kid = "CLIENT-UNSET"
	}

	now := time.Now().UTC().Unix()
	nb := make([]byte, 12)
	rand.Read(nb)
	nonce := base64.RawURLEncoding.EncodeToString(nb)

	payload := map[string]any{
		"v":     1,
		"kid":   kid,
		"iat":   now,
		"exp":   now + 90*24*60*60,
		"nonce": nonce,
	}
	pb, _ := json.Marshal(payload)

	sig := ed25519.Sign(priv, pb)

	buf := make([]byte, 0, 2+len(pb)+len(sig))
	tmp := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp, uint16(len(pb)))
	buf = append(buf, tmp...)
	buf = append(buf, pb...)
	buf = append(buf, sig...)

	keyStr := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(buf)
	out := group(keyStr, 4)

	fmt.Println("\nLICENSE KEY (give this to client):")
	fmt.Println(out)
	fmt.Printf("\nValid for 90 days from ACTIVATION on client's PC\nKeyID: %s\n", kid)
}

func group(s string, n int) string {
	var b strings.Builder
	for i := 0; i < len(s); i += n {
		if i > 0 {
			b.WriteString("-")
		}
		end := i + n
		if end > len(s) {
			end = len(s)
		}
		b.WriteString(s[i:end])
	}
	return b.String()
}