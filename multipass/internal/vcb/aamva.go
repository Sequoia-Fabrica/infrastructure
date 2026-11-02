package vcb

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
)

// MandatoryFields are the 22 fields every AAMVA DL/ID barcode carries, in
// the code point order protectedComponentIndex bits refer to.
var MandatoryFields = []string{
	"DAC", "DAD", "DAG", "DAI", "DAJ", "DAK", "DAQ", "DAU", "DAY", "DBA", "DBB",
	"DBC", "DBD", "DCA", "DCB", "DCD", "DCF", "DCG", "DCS", "DDE", "DDF", "DDG",
}

// Field is one element of an AAMVA subfile: a three-letter ID and its data.
type Field struct {
	ID, Value string
}

// Subfile is one subfile of an AAMVA barcode, e.g. "DL", "ID" or "ZZ".
type Subfile struct {
	Type   string
	Fields []Field
}

func (s Subfile) encode() string {
	parts := make([]string, len(s.Fields))
	for i, f := range s.Fields {
		parts[i] = f.ID + f.Value
	}
	return s.Type + strings.Join(parts, "\n") + "\r"
}

// AAMVAFile lays out an AAMVA barcode's text: the header naming the issuer
// (IIN) and the subfiles' offsets, then the subfiles.
func AAMVAFile(iin string, version, jurisdictionVersion int, subfiles []Subfile) (string, error) {
	if len(iin) != 6 {
		return "", errors.New("aamva: the IIN is six digits")
	}
	header := fmt.Sprintf("@\n\x1e\rANSI %s%02d%02d%02d", iin, version, jurisdictionVersion, len(subfiles))
	offset := len(header) + 10*len(subfiles)
	var dir, body strings.Builder
	for _, s := range subfiles {
		enc := s.encode()
		fmt.Fprintf(&dir, "%s%04d%04d", s.Type, offset, len(enc))
		body.WriteString(enc)
		offset += len(enc)
	}
	return header + dir.String() + body.String(), nil
}

// ProtectedComponentIndex marks which mandatory fields the signature
// covers: 22 bits and two of padding, multibase base64url.
func ProtectedComponentIndex(protected []string) (string, error) {
	var bits uint32
	for _, id := range protected {
		i := sort.SearchStrings(MandatoryFields, id)
		if i == len(MandatoryFields) || MandatoryFields[i] != id {
			return "", fmt.Errorf("aamva: %s is not a mandatory field", id)
		}
		bits |= 1 << (23 - i)
	}
	return "u" + base64.RawURLEncoding.EncodeToString([]byte{byte(bits >> 16), byte(bits >> 8), byte(bits)}), nil
}

// OpticalData is the barcode's protected fields as ecdsa-xi-2023 lays them
// out: each one's ID and data and a newline, in code point order.
func OpticalData(fields []Field, protected []string) ([]byte, error) {
	values := map[string]string{}
	for _, f := range fields {
		values[f.ID] = f.Value
	}
	ids := append([]string(nil), protected...)
	sort.Strings(ids)
	var b strings.Builder
	for _, id := range ids {
		v, ok := values[id]
		if !ok {
			return nil, fmt.Errorf("aamva: protected field %s is missing", id)
		}
		b.WriteString(id + v + "\n")
	}
	return []byte(b.String()), nil
}

// OpticalDataBytes is the spec's opticalDataBytes: the hash of the optical
// data. Signing hashes it once more, alongside the credential's hashes.
func OpticalDataBytes(fields []Field, protected []string) ([]byte, error) {
	data, err := OpticalData(fields, protected)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256(data)
	return h[:], nil
}

// Barcode is an AAMVA barcode to sign.
type Barcode struct {
	IIN     string
	Subfile Subfile // the DL or ID subfile, with every mandatory field
	Status  *StatusEntry
}

// StatusEntry places a credential in the issuer's status lists.
type StatusEntry struct {
	BaseURL string // the lists are at BaseURL/<purpose>/<list index>
	Index   uint32
}

// AAMVA versions: the 2020 Card Design Standard, no jurisdiction revisions.
const (
	aamvaVersion        = 10
	jurisdictionVersion = 0
)

// Sign returns the barcode's text: its subfile, then a ZZ subfile with a
// signed credential, in base64url, over every mandatory field.
func (iss Issuer) Sign(bc Barcode) (string, error) {
	for _, id := range MandatoryFields {
		found := false
		for _, f := range bc.Subfile.Fields {
			found = found || f.ID == id
		}
		if !found {
			return "", fmt.Errorf("aamva: mandatory field %s is missing", id)
		}
	}
	vc, err := iss.Credential(bc)
	if err != nil {
		return "", err
	}
	payload, err := EncodeCBORLD(vc, iss.Contexts)
	if err != nil {
		return "", err
	}
	zz := Subfile{Type: "ZZ", Fields: []Field{{"ZZA", base64.URLEncoding.EncodeToString(payload)}}}
	return AAMVAFile(bc.IIN, aamvaVersion, jurisdictionVersion, []Subfile{bc.Subfile, zz})
}

// Credential is the signed OpticalBarcodeCredential for a barcode.
func (iss Issuer) Credential(bc Barcode) (map[string]any, error) {
	index, err := ProtectedComponentIndex(MandatoryFields)
	if err != nil {
		return nil, err
	}
	optical, err := OpticalDataBytes(bc.Subfile.Fields, MandatoryFields)
	if err != nil {
		return nil, err
	}
	vc := map[string]any{
		"@context": []any{CredentialsV2, BarcodesV1},
		"type":     []any{"VerifiableCredential", "OpticalBarcodeCredential"},
		"issuer":   iss.DID,
		"credentialSubject": map[string]any{
			"type":                    "AamvaDriversLicenseScannableInformation",
			"protectedComponentIndex": index,
		},
	}
	if bc.Status != nil {
		vc["credentialStatus"] = map[string]any{
			"type":                   "TerseBitstringStatusListEntry",
			"terseStatusListBaseUrl": bc.Status.BaseURL,
			"terseStatusListIndex":   bc.Status.Index,
		}
	}
	return iss.sign(vc, optical)
}
