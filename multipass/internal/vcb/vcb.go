// Package vcb issues W3C Verifiable Credential Barcodes: an AAMVA-format
// PDF417 whose ZZ subfile carries a credential, signed with ecdsa-xi-2023,
// over the barcode's own fields. ID scanners read the fields; verifiers that
// check the signature learn who issued them, here the makerspace's DID, and
// can check its status list for badges reported lost.
//
// https://w3c.github.io/vc-barcodes/
package vcb

import (
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/piprate/json-gold/ld"
)

const (
	CredentialsV2 = "https://www.w3.org/ns/credentials/v2"
	BarcodesV1    = "https://w3id.org/vc-barcodes/v1"
)

//go:embed contexts/*.jsonld
var contextFiles embed.FS

// Contexts are JSON-LD context documents by URL. Nothing is fetched: a
// document using any other context is refused.
type Contexts map[string]map[string]any

// DefaultContexts are the contexts barcode and status list credentials use.
func DefaultContexts() Contexts {
	c := Contexts{}
	for url, file := range map[string]string{
		CredentialsV2: "contexts/credentials-v2.jsonld",
		BarcodesV1:    "contexts/vc-barcodes-v1.jsonld",
	} {
		b, err := contextFiles.ReadFile(file)
		if err != nil {
			panic(err)
		}
		if err := c.Add(url, b); err != nil {
			panic(err)
		}
	}
	return c
}

// Add parses a context document and makes it available at url.
func (c Contexts) Add(url string, doc []byte) error {
	var m map[string]any
	if err := json.Unmarshal(doc, &m); err != nil {
		return fmt.Errorf("context %s: %w", url, err)
	}
	c[url] = m
	return nil
}

func (c Contexts) LoadDocument(u string) (*ld.RemoteDocument, error) {
	doc, ok := c[u]
	if !ok {
		return nil, fmt.Errorf("context %s is not known offline", u)
	}
	return &ld.RemoteDocument{DocumentURL: u, Document: doc}, nil
}

// canonicalize returns the RDFC-1.0 canonical N-Quads of a JSON-LD document.
func (c Contexts) canonicalize(doc map[string]any) ([]byte, error) {
	// Round-trip through JSON so the processor sees only the types it knows
	raw, err := json.Marshal(doc)
	if err != nil {
		return nil, err
	}
	var plain map[string]any
	if err := json.Unmarshal(raw, &plain); err != nil {
		return nil, err
	}
	opts := ld.NewJsonLdOptions("")
	opts.Algorithm = ld.AlgorithmURDNA2015
	opts.Format = "application/n-quads"
	opts.ProcessingMode = ld.JsonLd_1_1
	opts.DocumentLoader = c
	opts.SafeMode = true
	out, err := ld.NewJsonLdProcessor().Normalize(plain, opts)
	if err != nil {
		return nil, fmt.Errorf("canonicalizing: %w", err)
	}
	s, _ := out.(string)
	if s == "" {
		return nil, errors.New("canonicalizing: document has no statements")
	}
	return []byte(s), nil
}

// Key is the issuer's P-256 signing key.
type Key struct {
	priv *ecdsa.PrivateKey
}

// DeriveKey derives a P-256 key from a secret, so there's no second secret to
// keep: label separates it from anything else derived from the same secret.
func DeriveKey(secret, label string) Key {
	n := elliptic.P256().Params().N
	for i := 0; ; i++ {
		mac := hmac.New(sha256.New, []byte(secret))
		fmt.Fprintf(mac, "%s:%d", label, i)
		d := new(big.Int).SetBytes(mac.Sum(nil))
		if d.Sign() == 0 || d.Cmp(n) >= 0 {
			continue // 1 in 2^32 odds
		}
		k, err := keyFromScalar(d)
		if err != nil {
			panic(err)
		}
		return k
	}
}

// GenerateKey makes a random key, for tests.
func GenerateKey() Key {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		panic(err)
	}
	return Key{priv}
}

func keyFromScalar(d *big.Int) (Key, error) {
	ek, err := ecdh.P256().NewPrivateKey(d.FillBytes(make([]byte, 32)))
	if err != nil {
		return Key{}, err
	}
	pub := ek.PublicKey().Bytes() // 0x04 || X || Y
	priv := &ecdsa.PrivateKey{D: d}
	priv.Curve = elliptic.P256()
	priv.X = new(big.Int).SetBytes(pub[1:33])
	priv.Y = new(big.Int).SetBytes(pub[33:])
	return Key{priv}, nil
}

// PublicKeyMultibase is the key in Multikey form: base58btc of the
// p256-pub multicodec and the compressed point, which starts "zDn".
func (k Key) PublicKeyMultibase() string {
	return publicKeyMultibase(&k.priv.PublicKey)
}

func publicKeyMultibase(pub *ecdsa.PublicKey) string {
	point := elliptic.MarshalCompressed(elliptic.P256(), pub.X, pub.Y)
	return "z" + base58Encode(append([]byte{0x80, 0x24}, point...))
}

// ParsePublicKeyMultibase reads a P-256 Multikey, as found in a did:key.
func ParsePublicKeyMultibase(s string) (*ecdsa.PublicKey, error) {
	if !strings.HasPrefix(s, "z") {
		return nil, errors.New("multikey: not base58btc")
	}
	b, err := base58Decode(s[1:])
	if err != nil {
		return nil, err
	}
	if len(b) != 35 || b[0] != 0x80 || b[1] != 0x24 {
		return nil, errors.New("multikey: not a compressed P-256 key")
	}
	x, y := elliptic.UnmarshalCompressed(elliptic.P256(), b[2:])
	if x == nil {
		return nil, errors.New("multikey: bad point")
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// Issuer is who signs: a DID and the verification method in its document
// that holds the key.
type Issuer struct {
	DID      string
	KeyID    string // full verification method ID, DID#fragment
	Key      Key
	Contexts Contexts
}

// DIDWeb is the did:web for a host, which resolves to
// https://<host>/.well-known/did.json.
func DIDWeb(host string) string {
	return "did:web:" + strings.ReplaceAll(host, ":", "%3A")
}

// DIDDocument is what https://<host>/.well-known/did.json serves for a
// did:web issuer: the one key, for issuing credentials.
func (iss Issuer) DIDDocument() map[string]any {
	return map[string]any{
		"@context": []string{"https://www.w3.org/ns/did/v1", "https://w3id.org/security/multikey/v1"},
		"id":       iss.DID,
		"verificationMethod": []any{map[string]any{
			"id":                 iss.KeyID,
			"type":               "Multikey",
			"controller":         iss.DID,
			"publicKeyMultibase": iss.Key.PublicKeyMultibase(),
		}},
		"assertionMethod": []string{iss.KeyID},
	}
}

// hashData is the ecdsa-rdfc-2019 hash of a document and proof options, with
// ecdsa-xi-2023's hash of the optical data after them when there is any.
func (c Contexts) hashData(unsecured, proofOptions map[string]any, opticalData []byte) ([]byte, error) {
	config := map[string]any{}
	for k, v := range proofOptions {
		if k != "proofValue" {
			config[k] = v
		}
	}
	config["@context"] = unsecured["@context"]
	canonConfig, err := c.canonicalize(config)
	if err != nil {
		return nil, err
	}
	canonDoc, err := c.canonicalize(unsecured)
	if err != nil {
		return nil, err
	}
	h1, h2 := sha256.Sum256(canonConfig), sha256.Sum256(canonDoc)
	out := append(h1[:], h2[:]...)
	if opticalData != nil {
		h3 := sha256.Sum256(opticalData)
		out = append(out, h3[:]...)
	}
	return out, nil
}

// sign adds a Data Integrity proof to doc. With opticalData (the
// OpticalDataBytes) it is ecdsa-xi-2023; without, ecdsa-rdfc-2019.
func (iss Issuer) sign(doc map[string]any, opticalData []byte) (map[string]any, error) {
	suite := "ecdsa-rdfc-2019"
	if opticalData != nil {
		suite = "ecdsa-xi-2023"
	}
	proof := map[string]any{
		"type":               "DataIntegrityProof",
		"cryptosuite":        suite,
		"proofPurpose":       "assertionMethod",
		"verificationMethod": iss.KeyID,
	}
	data, err := iss.Contexts.hashData(doc, proof, opticalData)
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	r, s, err := ecdsa.Sign(rand.Reader, iss.Key.priv, digest[:])
	if err != nil {
		return nil, err
	}
	sig := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	proof["proofValue"] = "z" + base58Encode(sig)

	signed := map[string]any{}
	for k, v := range doc {
		signed[k] = v
	}
	signed["proof"] = proof
	return signed, nil
}

// Verify checks a credential's proof against pub. opticalData is the
// barcode's OpticalDataBytes for ecdsa-xi-2023, nil for ecdsa-rdfc-2019.
func (c Contexts) Verify(signed map[string]any, opticalData []byte, pub *ecdsa.PublicKey) error {
	proof, ok := signed["proof"].(map[string]any)
	if !ok {
		return errors.New("no proof")
	}
	pv, _ := proof["proofValue"].(string)
	if !strings.HasPrefix(pv, "z") {
		return errors.New("proofValue is not base58btc")
	}
	sig, err := base58Decode(pv[1:])
	if err != nil || len(sig) != 64 {
		return errors.New("bad proofValue")
	}
	unsecured := map[string]any{}
	for k, v := range signed {
		if k != "proof" {
			unsecured[k] = v
		}
	}
	data, err := c.hashData(unsecured, proof, opticalData)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(data)
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(pub, digest[:], r, s) {
		return errors.New("signature does not verify")
	}
	return nil
}

const base58Alphabet = "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"

func base58Encode(b []byte) string {
	n := new(big.Int).SetBytes(b)
	base, mod := big.NewInt(58), new(big.Int)
	var out []byte
	for n.Sign() > 0 {
		n.DivMod(n, base, mod)
		out = append(out, base58Alphabet[mod.Int64()])
	}
	for _, c := range b {
		if c != 0 {
			break
		}
		out = append(out, '1')
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return string(out)
}

func base58Decode(s string) ([]byte, error) {
	n := new(big.Int)
	base := big.NewInt(58)
	for _, r := range s {
		i := strings.IndexRune(base58Alphabet, r)
		if i < 0 {
			return nil, fmt.Errorf("base58: bad character %q", r)
		}
		n.Mul(n, base).Add(n, big.NewInt(int64(i)))
	}
	out := n.Bytes()
	zeros := 0
	for zeros < len(s) && s[zeros] == '1' {
		zeros++
	}
	return append(make([]byte, zeros), out...), nil
}
