package badge

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"

	"multipass/internal/vcb"
)

// The back of a badge carries an AAMVA-format PDF417, the layout ID scanners
// read, signed as a W3C Verifiable Credential Barcode with the makerspace's
// own key. Scanners show the name and member number; a verifier that checks
// the signature sees it was issued by the makerspace's DID, not a state.
//
// The fields say only what a badge is. There's no date of birth, height or
// address, and no expiry: those read as "unavl", so an age check fails
// rather than passing on a makerspace badge. The subfile is "ID", which
// claims no driving privileges, the jurisdiction is "XX", which names none,
// and the IIN is the makerspace's own (see IIN).

// IIN is the six-digit issuer number in the barcode's header, which tells a
// scanner who issued the card. AAMVA jurisdictions all hold IINs beginning
// with 6, so a 9 and five digits drawn from the issuer's DID gives a number
// no state uses, and the same one on every badge from this multipass.
func IIN(issuerDID string) string {
	h := sha256.Sum256([]byte(issuerDID))
	n := (uint32(h[0])<<16 | uint32(h[1])<<8 | uint32(h[2])) % 100000
	return fmt.Sprintf("9%05d", n)
}

// DocumentDiscriminator is the barcode's per-card number (DCF): the serial
// and the QR code's signature, so a found card can be traced to its record.
func DocumentDiscriminator(serial int, sig string) string {
	return fmt.Sprintf("B%d-%s", serial, sig)
}

// AAMVA field limits for names, in the 2020 standard.
const (
	maxFamilyName = 40
	maxFirstName  = 40
	maxMiddleName = 40
)

var nameStrip = regexp.MustCompile(`[^A-Z0-9 '-]+`)

// BarcodeFields are the ID subfile of a badge's PDF417.
func BarcodeFields(rec Record, sig string) []vcb.Field {
	words := strings.Fields(nameStrip.ReplaceAllString(foldText(rec.Name), " "))
	first, middle, family := "NONE", "NONE", "NONE"
	switch len(words) {
	case 0:
	case 1:
		family = words[0]
	default:
		first, family = words[0], words[len(words)-1]
		if len(words) > 2 {
			middle = strings.Join(words[1:len(words)-1], " ")
		}
	}
	family, famT := truncate(family, maxFamilyName)
	first, firstT := truncate(first, maxFirstName)
	middle, midT := truncate(middle, maxMiddleName)

	return []vcb.Field{
		{ID: "DCA", Value: "NONE"}, // vehicle class, restrictions, endorsements:
		{ID: "DCB", Value: "NONE"}, // a badge carries no driving privileges
		{ID: "DCD", Value: "NONE"},
		{ID: "DBA", Value: "unavl"}, // expiry: badges don't expire, they're voided
		{ID: "DCS", Value: family},
		{ID: "DAC", Value: first},
		{ID: "DAD", Value: middle},
		{ID: "DBD", Value: rec.IssuedAt.UTC().Format("01022006")},
		{ID: "DBB", Value: "unavl"}, // date of birth: deliberately absent
		{ID: "DBC", Value: "9"},     // sex: not specified
		{ID: "DAY", Value: "UNK"},
		{ID: "DAU", Value: "unavl"},
		{ID: "DAG", Value: "unavl"},
		{ID: "DAI", Value: "unavl"},
		{ID: "DAJ", Value: "XX"},
		{ID: "DAK", Value: "unavl"},
		{ID: "DAQ", Value: rec.MemberID},
		{ID: "DCF", Value: DocumentDiscriminator(rec.Serial, sig)},
		{ID: "DCG", Value: "USA"},
		{ID: "DDE", Value: famT},
		{ID: "DDF", Value: firstT},
		{ID: "DDG", Value: midT},
	}
}

func truncate(s string, max int) (string, string) {
	if len(s) <= max {
		return s, "N"
	}
	return strings.TrimSpace(s[:max]), "T"
}

var contexts = sync.OnceValue(vcb.DefaultContexts)

// Issuer is the makerspace as a credential issuer: did:web on the public
// host, with a P-256 key derived from the badge secret.
func Issuer(secret, publicURL string) (vcb.Issuer, error) {
	host, err := Host(publicURL)
	if err != nil {
		return vcb.Issuer{}, err
	}
	did := vcb.DIDWeb(host)
	return vcb.Issuer{
		DID:      did,
		KeyID:    did + "#badge-key-1",
		Key:      vcb.DeriveKey(secret, "multipass-badge-vcb-p256:v1"),
		Contexts: contexts(),
	}, nil
}

// StatusBaseURL is where the badge status lists are published, as
// <base>/revocation/<list>.
func StatusBaseURL(publicURL string) string {
	return strings.TrimRight(publicURL, "/") + "/badge-status"
}

// BarcodeText is the signed AAMVA text for a badge's PDF417.
func BarcodeText(iss vcb.Issuer, publicURL string, rec Record, sig string) (string, error) {
	return iss.Sign(vcb.Barcode{
		IIN:     IIN(iss.DID),
		Subfile: vcb.Subfile{Type: "ID", Fields: BarcodeFields(rec, sig)},
		Status:  &vcb.StatusEntry{BaseURL: StatusBaseURL(publicURL), Index: rec.StatusIndex},
	})
}
