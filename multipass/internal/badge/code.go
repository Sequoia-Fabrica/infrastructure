// Package badge makes the laser-engraving files for wooden member badges,
// signs the codes they carry, and keeps the record of every badge issued.
package badge

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"net/url"
	"strings"
)

// SigLength is how many base32 characters of the HMAC a badge's QR code
// carries (65 bits).
const SigLength = 13

var sigEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// Sign returns the signature in the QR code of badge serial, issued to
// memberID. It never expires; a badge stops working when its record is voided.
func Sign(serial int, memberID string, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	fmt.Fprintf(mac, "multipass-badge:v2:%d:%s", serial, memberID)
	return sigEncoding.EncodeToString(mac.Sum(nil))[:SigLength]
}

// Verify reports whether sig is the signature of badge serial issued to
// memberID. Signatures are case-insensitive, since they travel in an
// uppercase QR code.
func Verify(serial int, memberID, sig, secret string) bool {
	if serial <= 0 || !IsMemberID(memberID) || len(sig) != SigLength {
		return false
	}
	want := Sign(serial, memberID, secret)
	return hmac.Equal([]byte(strings.ToUpper(sig)), []byte(want))
}

// IsMemberID reports whether id looks like an Authentik user PK.
func IsMemberID(id string) bool {
	if id == "" || len(id) > 12 {
		return false
	}
	for _, r := range id {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// URL is what a badge's QR code encodes. The scheme, host and path are all
// uppercase so the QR encoder can use alphanumeric mode, which needs fewer,
// larger modules than byte mode: easier to burn into wood, easier to scan.
func URL(baseURL string, serial int, sig string) (string, error) {
	host, err := Host(baseURL)
	if err != nil {
		return "", err
	}
	u, _ := url.Parse(baseURL)
	return strings.ToUpper(fmt.Sprintf("%s://%s/B/%d/%s", u.Scheme, host, serial, sig)), nil
}

// Host is the host part of the badge base URL, which must be nothing more
// than a scheme and host: a path would be uppercased along with the rest.
func Host(baseURL string) (string, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return "", err
	}
	if u.Scheme == "" || u.Host == "" || u.Path != "" {
		return "", fmt.Errorf("badge base URL %q must be just a scheme and host", baseURL)
	}
	return u.Host, nil
}
