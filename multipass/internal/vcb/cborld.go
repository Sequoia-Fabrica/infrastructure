package vcb

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
)

// This is a CBOR-LD encoder for the one processing model VC barcodes use:
// registry entry 100, semantic compression, and the URL and multibase
// codecs. It covers what the barcode and status credentials need and refuses
// anything else, rather than guess at bytes another implementation would
// decode differently. https://w3c.github.io/cbor-ld/

const (
	cborldTag      = 0xCB1D // 51997, the CBOR-LD tag
	vcbRegistryID  = 100    // the VC barcodes registry entry
	firstTermID    = 100
	multibaseIRI   = "https://w3id.org/security#multibase"
	cryptosuiteIRI = "https://w3id.org/security#cryptosuiteString"
)

var keywordIDs = map[string]int{
	"@context": 0, "@type": 2, "@id": 4, "@value": 6, "@direction": 8,
	"@graph": 10, "@included": 12, "@index": 14, "@json": 16, "@language": 18,
	"@list": 20, "@nest": 22, "@reverse": 24, "@base": 26, "@container": 28,
	"@default": 30, "@embed": 32, "@explicit": 34, "@none": 36,
	"@omitDefault": 38, "@prefix": 40, "@preserve": 42, "@protected": 44,
	"@requireAll": 46, "@set": 48, "@version": 50, "@vocab": 52, "@propagate": 54,
}

// typeTable is registry entry 100's, plus the contexts in use, which take
// IDs from 32768 in the order they're given.
func typeTable(contextURLs []string) map[string]map[string]int {
	ctx := map[string]int{}
	for i, u := range contextURLs {
		ctx[u] = 32768 + i
	}
	return map[string]map[string]int{
		"context": ctx,
		cryptosuiteIRI: {
			"ecdsa-rdfc-2019": 1,
			"ecdsa-sd-2023":   2,
			"eddsa-rdfc-2022": 3,
			"ecdsa-xi-2023":   4,
		},
	}
}

type termDef struct {
	id      string // @id
	typ     string // @type
	context any    // scoped @context
}

type cborldEncoder struct {
	contexts  Contexts
	typeTable map[string]map[string]int
	termToID  map[string]int
	nextID    int
	defs      map[string]termDef
}

// EncodeCBORLD compresses a JSON-LD document whose @context is a list of
// context URLs known to contexts, all of which are in typeTable's context
// table in that order.
func EncodeCBORLD(doc map[string]any, contexts Contexts) ([]byte, error) {
	urls, err := contextURLs(doc)
	if err != nil {
		return nil, err
	}
	e := &cborldEncoder{
		contexts:  contexts,
		typeTable: typeTable(urls),
		termToID:  map[string]int{},
		nextID:    firstTermID,
		defs:      map[string]termDef{},
	}
	for k, v := range keywordIDs {
		e.termToID[k] = v
	}
	body, err := e.object(doc, true)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	writeHead(&out, 6, cborldTag)
	writeHead(&out, 4, 2)
	writeHead(&out, 0, vcbRegistryID)
	out.Write(body)
	return out.Bytes(), nil
}

func contextURLs(doc map[string]any) ([]string, error) {
	var urls []string
	switch c := doc["@context"].(type) {
	case string:
		urls = []string{c}
	case []string:
		urls = c
	case []any:
		for _, u := range c {
			s, ok := u.(string)
			if !ok {
				return nil, errors.New("cbor-ld: only context URLs are supported")
			}
			urls = append(urls, s)
		}
	default:
		return nil, errors.New("cbor-ld: document has no @context")
	}
	return urls, nil
}

// load adds a context's terms, sorted, to the term IDs, and their
// definitions to the active ones.
func (e *cborldEncoder) load(ctx any) error {
	if u, ok := ctx.(string); ok {
		c, ok := e.contexts[u]
		if !ok {
			return fmt.Errorf("cbor-ld: unknown context %s", u)
		}
		ctx = c["@context"]
	}
	m, ok := ctx.(map[string]any)
	if !ok {
		return fmt.Errorf("cbor-ld: unsupported context %v", ctx)
	}
	if _, ok := m["@import"]; ok {
		return errors.New("cbor-ld: @import is not supported")
	}
	terms := make([]string, 0, len(m))
	for t := range m {
		terms = append(terms, t)
	}
	sort.Strings(terms)
	for _, term := range terms {
		if _, ok := keywordIDs[term]; ok {
			continue
		}
		var def termDef
		switch d := m[term].(type) {
		case nil:
			continue
		case string:
			def.id = d
		case map[string]any:
			def.id, _ = d["@id"].(string)
			def.typ, _ = d["@type"].(string)
			def.context = d["@context"]
		default:
			return fmt.Errorf("cbor-ld: bad definition for %s", term)
		}
		e.defs[term] = def
		if _, ok := e.termToID[term]; !ok {
			e.termToID[term] = e.nextID
			e.nextID += 2
		}
	}
	return nil
}

func (e *cborldEncoder) isTypeTerm(term string) bool {
	return term == "@type" || e.defs[term].id == "@type"
}

func (e *cborldEncoder) object(obj map[string]any, top bool) ([]byte, error) {
	entries := map[int][]byte{}

	if c, ok := obj["@context"]; ok {
		if !top {
			return nil, errors.New("cbor-ld: embedded contexts are only supported at the top")
		}
		urls, _ := contextURLs(obj)
		var enc bytes.Buffer
		for _, u := range urls {
			if err := e.load(u); err != nil {
				return nil, err
			}
			writeHead(&enc, 0, uint64(e.typeTable["context"][u]))
		}
		if _, single := c.(string); single {
			entries[0] = enc.Bytes()
		} else {
			var arr bytes.Buffer
			writeHead(&arr, 4, uint64(len(urls)))
			arr.Write(enc.Bytes())
			entries[1] = arr.Bytes()
		}
	}

	// Type-scoped contexts apply, in sorted order of type, before any of
	// the object's properties
	var types []string
	for k, v := range obj {
		if k == "@context" || !e.isTypeTerm(k) {
			continue
		}
		for _, t := range asList(v) {
			s, ok := t.(string)
			if !ok {
				return nil, fmt.Errorf("cbor-ld: %s must be a string", k)
			}
			types = append(types, s)
		}
	}
	sort.Strings(types)
	for _, t := range types {
		if def, ok := e.defs[t]; ok && def.context != nil {
			if err := e.load(def.context); err != nil {
				return nil, err
			}
		}
	}

	keys := make([]string, 0, len(obj))
	for k := range obj {
		if k != "@context" {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := obj[key]
		id, ok := e.termToID[key]
		if !ok {
			return nil, fmt.Errorf("cbor-ld: %q is not a term of the contexts", key)
		}
		def := e.defs[key]
		if def.context != nil {
			if err := e.load(def.context); err != nil {
				return nil, err
			}
		}
		list, plural := value.([]any)
		if !plural {
			if ss, ok := value.([]string); ok {
				plural = true
				for _, s := range ss {
					list = append(list, s)
				}
			}
		}
		var enc bytes.Buffer
		if plural {
			id++
			writeHead(&enc, 4, uint64(len(list)))
		} else {
			list = []any{value}
		}
		for _, v := range list {
			b, err := e.value(key, def, v)
			if err != nil {
				return nil, err
			}
			enc.Write(b)
		}
		entries[id] = enc.Bytes()
	}
	return encodeIntMap(entries), nil
}

func (e *cborldEncoder) value(term string, def termDef, v any) ([]byte, error) {
	if m, ok := v.(map[string]any); ok {
		return e.object(m, false)
	}
	var out bytes.Buffer
	isURL := e.isTypeTerm(term) || term == "@id" || def.id == "@id" || def.typ == "@id" || def.typ == "@vocab"
	tableType := "none"
	if isURL {
		tableType = "url"
	} else if def.typ != "" {
		tableType = def.typ
	}

	switch x := v.(type) {
	case bool:
		if x {
			out.WriteByte(0xf5)
		} else {
			out.WriteByte(0xf4)
		}
		return out.Bytes(), nil
	case int:
		if x < 0 {
			return nil, errors.New("cbor-ld: negative integers are not supported")
		}
		return e.integer(tableType, uint64(x))
	case uint32:
		return e.integer(tableType, uint64(x))
	case uint64:
		return e.integer(tableType, x)
	case float64:
		if x < 0 || x != math.Trunc(x) {
			return nil, errors.New("cbor-ld: only whole numbers are supported")
		}
		return e.integer(tableType, uint64(x))
	case string:
		if n, ok := e.typeTable[tableType][x]; ok {
			writeHead(&out, 0, uint64(n))
			return out.Bytes(), nil
		}
		switch tableType {
		case "url":
			return e.url(x)
		case multibaseIRI:
			return multibaseValue(x)
		case "none":
		default:
			if strings.HasPrefix(tableType, "http://www.w3.org/2001/XMLSchema#date") {
				return nil, fmt.Errorf("cbor-ld: %s values are not supported", tableType)
			}
		}
		writeText(&out, x)
		return out.Bytes(), nil
	}
	return nil, fmt.Errorf("cbor-ld: unsupported value %T for %s", v, term)
}

func (e *cborldEncoder) integer(tableType string, n uint64) ([]byte, error) {
	if tableType != "none" {
		// Typed integers go as signed byte strings; nothing here uses them
		return nil, fmt.Errorf("cbor-ld: typed integers (%s) are not supported", tableType)
	}
	var out bytes.Buffer
	writeHead(&out, 0, n)
	return out.Bytes(), nil
}

func (e *cborldEncoder) url(s string) ([]byte, error) {
	var out bytes.Buffer
	if id, ok := e.termToID[s]; ok {
		writeHead(&out, 0, uint64(id))
		return out.Bytes(), nil
	}
	for prefix, n := range map[string]uint64{"https://": 2, "http://": 1} {
		if rest, ok := strings.CutPrefix(s, prefix); ok && !strings.Contains(rest, ":") {
			writeHead(&out, 4, 2)
			writeHead(&out, 0, n)
			writeText(&out, rest)
			return out.Bytes(), nil
		}
	}
	for _, prefix := range []string{"urn:uuid:", "data:", "did:v1:nym:", "did:key:"} {
		if strings.HasPrefix(s, prefix) {
			return nil, fmt.Errorf("cbor-ld: %s URLs are not supported", prefix)
		}
	}
	writeText(&out, s)
	return out.Bytes(), nil
}

func multibaseValue(s string) ([]byte, error) {
	var payload []byte
	var err error
	switch {
	case strings.HasPrefix(s, "z"):
		payload, err = base58Decode(s[1:])
	case strings.HasPrefix(s, "u"):
		payload, err = base64.RawURLEncoding.DecodeString(s[1:])
	case strings.HasPrefix(s, "M"):
		payload, err = base64.StdEncoding.DecodeString(s[1:])
	default:
		var out bytes.Buffer
		writeText(&out, s)
		return out.Bytes(), nil
	}
	if err != nil {
		return nil, fmt.Errorf("cbor-ld: bad multibase %q: %w", s, err)
	}
	var out bytes.Buffer
	writeHead(&out, 2, uint64(1+len(payload)))
	out.WriteByte(s[0])
	out.Write(payload)
	return out.Bytes(), nil
}

func asList(v any) []any {
	switch x := v.(type) {
	case []any:
		return x
	case []string:
		l := make([]any, len(x))
		for i, s := range x {
			l[i] = s
		}
		return l
	}
	return []any{v}
}

// Minimal deterministic CBOR (RFC 8949 section 4.2.1): shortest heads, and
// map keys in the bytewise order of their encodings.

func writeHead(b *bytes.Buffer, major byte, n uint64) {
	m := major << 5
	switch {
	case n < 24:
		b.WriteByte(m | byte(n))
	case n <= math.MaxUint8:
		b.Write([]byte{m | 24, byte(n)})
	case n <= math.MaxUint16:
		b.Write([]byte{m | 25, byte(n >> 8), byte(n)})
	case n <= math.MaxUint32:
		b.Write([]byte{m | 26, byte(n >> 24), byte(n >> 16), byte(n >> 8), byte(n)})
	default:
		b.WriteByte(m | 27)
		for i := 7; i >= 0; i-- {
			b.WriteByte(byte(n >> (8 * i)))
		}
	}
}

func writeText(b *bytes.Buffer, s string) {
	writeHead(b, 3, uint64(len(s)))
	b.WriteString(s)
}

func encodeIntMap(entries map[int][]byte) []byte {
	type kv struct{ k, v []byte }
	list := make([]kv, 0, len(entries))
	for k, v := range entries {
		var kb bytes.Buffer
		writeHead(&kb, 0, uint64(k))
		list = append(list, kv{kb.Bytes(), v})
	}
	sort.Slice(list, func(i, j int) bool { return bytes.Compare(list[i].k, list[j].k) < 0 })
	var out bytes.Buffer
	writeHead(&out, 5, uint64(len(list)))
	for _, e := range list {
		out.Write(e.k)
		out.Write(e.v)
	}
	return out.Bytes()
}
