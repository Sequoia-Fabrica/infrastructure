package vcb

import (
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// The Utopia driver's licence from the spec's test vectors.
// https://w3c.github.io/vc-barcodes/#test-vectors

const utopiaV2 = "https://w3id.org/utopia/v2"

var utopiaFields = []Field{
	{"DAQ", "F987654321"}, {"DCS", "SMITH"}, {"DDE", "N"}, {"DAC", "JOHN"},
	{"DDF", "N"}, {"DAD", "NONE"}, {"DDG", "N"}, {"DCA", "C"}, {"DCB", "NONE"},
	{"DCD", "NONE"}, {"DBD", "01012024"}, {"DBB", "04191988"}, {"DBA", "04192030"},
	{"DBC", "1"}, {"DAU", "069 IN"}, {"DAY", "BRO"}, {"DAG", "123 MAIN ST"},
	{"DAI", "ANYVILLE"}, {"DAJ", "UTO"}, {"DAK", "F87P20000  "},
	{"DCF", "UTODOCDISCRIM"}, {"DCG", "UTO"}, {"DAW", "158"}, {"DCK", "1234567890"},
	{"DDA", "N"},
}

const utopiaDID = "did:key:zDnaeWjKfs1ob9QcgasjYSPEMkwq31hmvSAWPVAgnrt1e9GKj"

func utopiaVC() map[string]any {
	return map[string]any{
		"@context": []any{CredentialsV2, BarcodesV1, utopiaV2},
		"type":     []any{"VerifiableCredential", "OpticalBarcodeCredential"},
		"credentialSubject": map[string]any{
			"type":                    "AamvaDriversLicenseScannableInformation",
			"protectedComponentIndex": "uggAg",
		},
		"issuer": utopiaDID,
		"credentialStatus": map[string]any{
			"type":                   "TerseBitstringStatusListEntry",
			"terseStatusListBaseUrl": "https://sandbox.platform.veres.dev/statuses/z19rJ4oGrbFCqf3cNTVDHSbNd/status-lists",
			"terseStatusListIndex":   uint64(3851559041),
		},
		"proof": map[string]any{
			"type":               "DataIntegrityProof",
			"verificationMethod": utopiaDID + "#zDnaeWjKfs1ob9QcgasjYSPEMkwq31hmvSAWPVAgnrt1e9GKj",
			"cryptosuite":        "ecdsa-xi-2023",
			"proofPurpose":       "assertionMethod",
			"proofValue":         "z4g6G3dAZhhtPxPWgFvkiRv7krtCaeJxjokvL46fchAFCXEY3FeX2vn46MDgBaw779g1E1jswZJxxreZDCrtHg2qH",
		},
	}
}

func utopiaContexts(t *testing.T) Contexts {
	c := DefaultContexts()
	b, err := os.ReadFile("testdata/utopia-v2.jsonld")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Add(utopiaV2, b); err != nil {
		t.Fatal(err)
	}
	return c
}

var utopiaProtected = []string{"DAC", "DAQ", "DCS"}

func TestProtectedComponentIndex(t *testing.T) {
	if got, _ := ProtectedComponentIndex(utopiaProtected); got != "uggAg" {
		t.Errorf("index = %q, want uggAg", got)
	}
	if got, _ := ProtectedComponentIndex(MandatoryFields); got != "u___8" {
		t.Errorf("all fields = %q, want u___8", got)
	}
	if _, err := ProtectedComponentIndex([]string{"DAW"}); err == nil {
		t.Error("an optional field was accepted")
	}
}

func TestOpticalDataMatchesSpec(t *testing.T) {
	data, err := OpticalData(utopiaFields, utopiaProtected)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "DACJOHN\nDAQF987654321\nDCSSMITH\n" {
		t.Errorf("optical data = %q", data)
	}
	want := []byte{188, 38, 200, 146, 227, 213, 90, 250, 50, 18, 126, 254, 47, 177, 91, 23,
		64, 129, 104, 223, 136, 81, 116, 67, 136, 125, 137, 165, 117, 63, 152, 207}
	if h := sha256.Sum256(data); !bytes.Equal(h[:], want) {
		t.Errorf("hash = %v", h)
	}
}

func TestCBORLDMatchesSpec(t *testing.T) {
	got, err := EncodeCBORLD(utopiaVC(), utopiaContexts(t))
	if err != nil {
		t.Fatal(err)
	}
	want := "D9CB1D821864A60183198000198001198002189D82187618A418B8A3189C18A618CE18B218D01AE592208118BAA2189C18A018A8447582002018BE18AA18C0A5189C186C18D60418E018E618E258417AB7C2E56B49E2CCE62184CE26818E15A8B173164401B5D3BB93FFD6D2B5EB8F6AC0971502AE3DD49D17EC66528164034C912685B8111BC04CDC9EC13DBADD91CC18E418AC"
	if strings.ToUpper(hex.EncodeToString(got)) != want {
		t.Errorf("CBOR-LD\n got %X\nwant %s", got, want)
	}
}

func TestVerifiesSpecSignature(t *testing.T) {
	pub, err := ParsePublicKeyMultibase(strings.TrimPrefix(utopiaDID, "did:key:"))
	if err != nil {
		t.Fatal(err)
	}
	data, _ := OpticalDataBytes(utopiaFields, utopiaProtected)
	c := utopiaContexts(t)
	if err := c.Verify(utopiaVC(), data, pub); err != nil {
		t.Fatalf("spec credential: %v", err)
	}
	fields := append([]Field(nil), utopiaFields...)
	fields[1].Value = "SMYTH"
	tampered, _ := OpticalDataBytes(fields, utopiaProtected)
	if err := c.Verify(utopiaVC(), tampered, pub); err == nil {
		t.Error("verified with a changed surname")
	}
}

func TestAAMVAFileMatchesSpec(t *testing.T) {
	zza := "2csdghhkpgGDGYAAGYABGYACGJ2CGHYYpBi4oxicGKYYzhiyGNAa5ZIggRi6ohicGKAYqER1ggAgGL4YqhjApRicGGwY1gQY4BjmGOJYQXq3wuVrSeLM5iGEziaBjhWosXMWRAG107uT/9bSteuPasCXFQKuPdSdF+xmUoFkA0yRJoW4ERvATNyewT263ZHMGOQYrA=="
	got, err := AAMVAFile("000000", 9, 0, []Subfile{
		{Type: "DL", Fields: utopiaFields},
		{Type: "ZZ", Fields: []Field{{"ZZA", zza}}},
	})
	if err != nil {
		t.Fatal(err)
	}
	// The spec gives the ZZ subfile's length as 0202, though its own ZZA
	// text makes it 206 bytes; everything else matches
	want := "@\n\x1e\rANSI 000000090002DL00410234ZZ02750206DLDAQF987654321\nDCSSMITH\nDDEN\nDACJOHN\nDDFN\nDADNONE\nDDGN\nDCAC\nDCBNONE\nDCDNONE\nDBD01012024\nDBB04191988\nDBA04192030\nDBC1\nDAU069 IN\nDAYBRO\nDAG123 MAIN ST\nDAIANYVILLE\nDAJUTO\nDAKF87P20000  \nDCFUTODOCDISCRIM\nDCGUTO\nDAW158\nDCK1234567890\nDDAN\rZZZZA" + zza + "\r"
	if got != want {
		t.Errorf("AAMVA file\n got %q\nwant %q", got, want)
	}
	// And the ZZA really is the spec's CBOR-LD
	payload, _ := base64.StdEncoding.DecodeString(zza)
	if !strings.HasPrefix(strings.ToUpper(hex.EncodeToString(payload)), "D9CB1D821864") {
		t.Errorf("ZZA is not CBOR-LD: %X", payload)
	}
}

func testIssuer() Issuer {
	key := GenerateKey()
	did := DIDWeb("multipass.example.org")
	return Issuer{DID: did, KeyID: did + "#key-1", Key: key, Contexts: DefaultContexts()}
}

func TestSignAndVerifyBarcode(t *testing.T) {
	iss := testIssuer()
	bc := Barcode{
		IIN:     "000000",
		Subfile: Subfile{Type: "ID", Fields: utopiaFields},
		Status:  &StatusEntry{BaseURL: "https://multipass.example.org/badge-status", Index: 123456789},
	}
	text, err := iss.Sign(bc)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(text, "@\n\x1e\rANSI 000000100002ID0041") || !strings.Contains(text, "\rZZZZA2csd") {
		t.Errorf("unexpected barcode %q", text)
	}

	vc, err := iss.Credential(bc)
	if err != nil {
		t.Fatal(err)
	}
	all, _ := OpticalDataBytes(utopiaFields, MandatoryFields)
	if err := iss.Contexts.Verify(vc, all, &iss.Key.priv.PublicKey); err != nil {
		t.Fatalf("own credential: %v", err)
	}
	if err := iss.Contexts.Verify(vc, all, &GenerateKey().priv.PublicKey); err == nil {
		t.Error("verified with someone else's key")
	}
	if _, err := EncodeCBORLD(vc, iss.Contexts); err != nil {
		t.Fatal(err)
	}

	missing := bc
	missing.Subfile.Fields = utopiaFields[1:]
	if _, err := iss.Sign(missing); err == nil {
		t.Error("signed a barcode missing a mandatory field")
	}
}

func TestStatusList(t *testing.T) {
	iss := testIssuer()
	idx := uint32(2*ListLength + 12345)
	vc, err := iss.StatusListCredential("https://multipass.example.org/badge-status", "revocation", 2,
		[]uint32{idx, 7 /* list 0 */}, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if err := iss.Contexts.Verify(vc, nil, &iss.Key.priv.PublicKey); err != nil {
		t.Fatalf("status list: %v", err)
	}
	if got := StatusListURL("https://multipass.example.org/badge-status", "revocation", idx); got != vc["id"] {
		t.Errorf("list URL %s, credential %s", got, vc["id"])
	}
	enc := vc["credentialSubject"].(map[string]any)["encodedList"].(string)
	z, _ := base64.RawURLEncoding.DecodeString(enc[1:])
	r, err := gzip.NewReader(bytes.NewReader(z))
	if err != nil {
		t.Fatal(err)
	}
	bits, _ := io.ReadAll(r)
	if len(bits) != ListLength/8 {
		t.Fatalf("list is %d bytes", len(bits))
	}
	set := 0
	for i, b := range bits {
		for j := 0; j < 8; j++ {
			if b&(0x80>>j) != 0 {
				set++
				if i*8+j != 12345 {
					t.Errorf("bit %d set", i*8+j)
				}
			}
		}
	}
	if set != 1 {
		t.Errorf("%d bits set, want 1", set)
	}
}

func TestDerivedKeyIsStable(t *testing.T) {
	a, b := DeriveKey("secret", "label"), DeriveKey("secret", "label")
	if a.PublicKeyMultibase() != b.PublicKeyMultibase() || !strings.HasPrefix(a.PublicKeyMultibase(), "zDn") {
		t.Errorf("keys %s %s", a.PublicKeyMultibase(), b.PublicKeyMultibase())
	}
	if DeriveKey("secret", "other").PublicKeyMultibase() == a.PublicKeyMultibase() {
		t.Error("labels share a key")
	}
	pub, err := ParsePublicKeyMultibase(a.PublicKeyMultibase())
	if err != nil || pub.X.Cmp(a.priv.X) != 0 {
		t.Errorf("multikey round trip: %v", err)
	}
}
