package license

import (
	"crypto/sha256"
	"encoding/hex"
	"os/exec"
	"strings"
)

func GetMachineID() string {
	var parts []string

	if out, err := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Cryptography`, "/v", "MachineGuid").CombinedOutput(); err == nil {
		s := string(out)
		if idx := strings.Index(s, "REG_SZ"); idx >= 0 {
			val := strings.TrimSpace(s[idx+6:])
			if val != "" {
				parts = append(parts, "mg:"+val)
			}
		}
	}

	if out, err := exec.Command("cmd", "/c", "vol C:").CombinedOutput(); err == nil {
		s := strings.ToLower(string(out))
		if strings.Contains(s, "serial number") {
			parts = append(parts, "volc:"+string(out))
		}
	}

	if len(parts) == 0 {
		parts = append(parts, "fallback:sabre-monitor")
	}

	h := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(h[:])
}
