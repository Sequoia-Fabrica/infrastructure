package vcb

import (
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"time"
)

// A TerseBitstringStatusListEntry's index spans 2^6 lists of 2^26 entries,
// the sizes the spec requires so an index alone says little about who holds
// a credential.
const (
	ListLength = 1 << 26
	ListCount  = 1 << 6
)

// RandomStatusIndex picks a status list index for a new credential.
func RandomStatusIndex() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	return binary.BigEndian.Uint32(b[:])
}

// StatusListURL is where a verifier finds the list holding index, for a
// purpose like "revocation".
func StatusListURL(baseURL, purpose string, index uint32) string {
	return fmt.Sprintf("%s/%s/%d", baseURL, purpose, index/ListLength)
}

// StatusListCredential is list number list of the issuer's status lists for
// purpose, with the bits of the given terse indexes set: a signed
// BitstringStatusListCredential (ecdsa-rdfc-2019).
func (iss Issuer) StatusListCredential(baseURL, purpose string, list int, set []uint32, now time.Time) (map[string]any, error) {
	if list < 0 || list >= ListCount {
		return nil, fmt.Errorf("status list %d does not exist", list)
	}
	bits := make([]byte, ListLength/8)
	for _, idx := range set {
		if int(idx/ListLength) != list {
			continue
		}
		i := idx % ListLength
		bits[i/8] |= 0x80 >> (i % 8)
	}
	var z bytes.Buffer
	gz := gzip.NewWriter(&z)
	if _, err := gz.Write(bits); err != nil {
		return nil, err
	}
	if err := gz.Close(); err != nil {
		return nil, err
	}

	id := fmt.Sprintf("%s/%s/%d", baseURL, purpose, list)
	return iss.sign(map[string]any{
		"@context":  []any{CredentialsV2},
		"id":        id,
		"type":      []any{"VerifiableCredential", "BitstringStatusListCredential"},
		"issuer":    iss.DID,
		"validFrom": now.UTC().Format(time.RFC3339),
		"credentialSubject": map[string]any{
			"id":            id + "#list",
			"type":          "BitstringStatusList",
			"statusPurpose": purpose,
			"encodedList":   "u" + base64.RawURLEncoding.EncodeToString(z.Bytes()),
		},
	}, nil)
}
