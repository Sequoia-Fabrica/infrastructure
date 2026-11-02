package badge

import (
	"encoding/xml"
	"io"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"multipass/internal/pdf417"
	"multipass/internal/vcb"
)

func TestSignAndVerify(t *testing.T) {
	sig := Sign(17, "42", "secret")
	if len(sig) != SigLength {
		t.Fatalf("signature %q is %d chars, want %d", sig, len(sig), SigLength)
	}
	if !Verify(17, "42", sig, "secret") {
		t.Error("fresh signature did not verify")
	}
	if !Verify(17, "42", strings.ToLower(sig), "secret") {
		t.Error("lowercased signature did not verify")
	}
	for name, ok := range map[string]bool{
		"other serial":   Verify(18, "42", sig, "secret"),
		"other member":   Verify(17, "43", sig, "secret"),
		"other secret":   Verify(17, "42", sig, "terces"),
		"truncated":      Verify(17, "42", sig[:SigLength-1], "secret"),
		"non-numeric id": Verify(17, "4a", Sign(17, "4a", "secret"), "secret"),
		"serial zero":    Verify(0, "42", Sign(0, "42", "secret"), "secret"),
	} {
		if ok {
			t.Errorf("%s: signature verified but should not have", name)
		}
	}
}

func TestURLIsAlphanumericQRSafe(t *testing.T) {
	u, err := URL("https://multipass.sequoia.garden/", 17, Sign(17, "42", "secret"))
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^HTTPS://MULTIPASS\.SEQUOIA\.GARDEN/B/17/[A-Z2-7]{13}$`).MatchString(u) {
		t.Errorf("unexpected badge URL %q", u)
	}
	// QR alphanumeric mode: 0-9, A-Z, space and $%*+-./:
	if strings.Trim(u, "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ $%*+-./:") != "" {
		t.Errorf("badge URL %q has characters outside QR alphanumeric mode", u)
	}
	if _, err := URL("https://example.org/multipass", 17, "X"); err == nil {
		t.Error("base URL with a path should be refused; its path would be uppercased")
	}
}

func TestBarcodeSaysOnlyWhatABadgeIs(t *testing.T) {
	iss, err := Issuer("secret", "https://multipass.sequoia.garden")
	if err != nil {
		t.Fatal(err)
	}
	if iss.DID != "did:web:multipass.sequoia.garden" {
		t.Errorf("issuer %s", iss.DID)
	}
	rec := Record{Serial: 17, MemberID: "42", Name: "Zoë Q. O'Brien-Smith",
		IssuedAt: time.Date(2026, 11, 1, 15, 4, 0, 0, time.UTC), StatusIndex: 3851559041}
	sig := Sign(17, "42", "secret")
	text, err := BarcodeText(iss, "https://multipass.sequoia.garden", rec, sig)
	if err != nil {
		t.Fatal(err)
	}

	iin := IIN(iss.DID)
	if !strings.HasPrefix(text, "@\n\x1e\rANSI "+iin+"100002ID") || iin[0] != '9' {
		t.Errorf("header %q: want our own 9xxxxx IIN and an ID subfile", text[:30])
	}
	for _, want := range []string{"\nDBBunavl\n", "\nDBAunavl\n", "\nDAJXX\n", "\nDAQ42\n",
		"\nDACZOE\n", "\nDADQ\n", "\nDCSO'BRIEN-SMITH\n", "\nDBD11012026\n", "\nDCFB17-" + sig + "\n", "\rZZZZA"} {
		if !strings.Contains(text, want) {
			t.Errorf("barcode lacks %q", want)
		}
	}

	// The signature covers every mandatory field
	pub, err := vcb.ParsePublicKeyMultibase(iss.Key.PublicKeyMultibase())
	if err != nil {
		t.Fatal(err)
	}
	fields := BarcodeFields(rec, sig)
	bc := vcb.Barcode{IIN: iin, Subfile: vcb.Subfile{Type: "ID", Fields: fields},
		Status: &vcb.StatusEntry{BaseURL: StatusBaseURL("https://multipass.sequoia.garden"), Index: rec.StatusIndex}}
	vc, err := iss.Credential(bc)
	if err != nil {
		t.Fatal(err)
	}
	optical, _ := vcb.OpticalDataBytes(fields, vcb.MandatoryFields)
	if err := iss.Contexts.Verify(vc, optical, pub); err != nil {
		t.Fatalf("badge credential: %v", err)
	}
	for i := range fields {
		if fields[i].ID == "DBB" {
			fields[i].Value = "01011990"
		}
	}
	forged, _ := vcb.OpticalDataBytes(fields, vcb.MandatoryFields)
	if err := iss.Contexts.Verify(vc, forged, pub); err == nil {
		t.Error("verified with a date of birth written in")
	}

	// Long names are cut to AAMVA's limits and flagged
	rec.Name = strings.Repeat("Bartholomew", 5) + " Smith"
	for _, f := range BarcodeFields(rec, sig) {
		if (f.ID == "DAC" && len(f.Value) > maxFirstName) || (f.ID == "DDF" && f.Value != "T") {
			t.Errorf("long first name: %s = %q", f.ID, f.Value)
		}
	}
}

func TestStoreLifecycle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "badges.json")
	s, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)

	first, err := s.Issue("42", "Ada", "ada@example.org", now)
	if err != nil || first.Serial != 1 || !first.Active() {
		t.Fatalf("Issue = %+v, %v", first, err)
	}
	if voided, _ := s.Voided(); len(voided) != 0 {
		t.Errorf("voided before any were: %v", voided)
	}
	_, before := s.Voided()
	if _, err := s.Issue("42", "Ada", "ada@example.org", now); err != ErrActiveBadge {
		t.Errorf("second active badge: err = %v, want ErrActiveBadge", err)
	}
	if _, err := s.Issue("43", "Bob", "bob@example.org", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Void(first.Serial, Failed, "ada@example.org", now); err != nil {
		t.Fatal(err)
	}
	if voided, after := s.Voided(); len(voided) != 1 || voided[0] != first.StatusIndex || after == before {
		t.Errorf("Voided = %v (version %d -> %d), want [%d]", voided, before, after, first.StatusIndex)
	}
	if _, err := s.Void(first.Serial, Lost, "ada@example.org", now); err != ErrNotActive {
		t.Errorf("voiding twice: err = %v, want ErrNotActive", err)
	}
	if _, err := s.Void(first.Serial, Lasered, "ada@example.org", now); err == nil {
		t.Error("voiding as lasered should be refused")
	}
	second, err := s.Issue("42", "Ada", "ada@example.org", now)
	if err != nil || second.Serial != 3 {
		t.Fatalf("reissue = %+v, %v", second, err)
	}

	// Everything survives a reopen
	s, err = OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	got := s.ForMember("42")
	if len(got) != 2 || got[0].Serial != 3 || got[1].Status != Failed || got[1].VoidedBy != "ada@example.org" {
		t.Errorf("ForMember after reopen = %+v", got)
	}
	if r, ok := s.Get(2); !ok || r.MemberID != "43" {
		t.Errorf("Get(2) = %+v, %v", r, ok)
	}
	next, _ := s.Issue("44", "Cy", "staff@example.org", now)
	if next.Serial != 4 {
		t.Errorf("serials restarted after reopen: got %d", next.Serial)
	}
	seen := map[uint32]bool{}
	for _, m := range []string{"42", "43", "44"} {
		for _, r := range s.ForMember(m) {
			if seen[r.StatusIndex] {
				t.Errorf("status index %d reused", r.StatusIndex)
			}
			seen[r.StatusIndex] = true
		}
	}
}

func TestFontCoversNamesAndStaysInItsBox(t *testing.T) {
	for r, g := range font {
		for _, st := range g.strokes {
			for _, p := range st {
				if p.X < -0.5 || p.X > g.width+0.7 || p.Y < -1.5 || p.Y > 10.5 {
					t.Errorf("glyph %q strays to (%.2f, %.2f)", r, p.X, p.Y)
				}
			}
		}
	}
	if got := foldText("Zoë Ångström-O'Brien"); got != "ZOE ANGSTROM-O'BRIEN" {
		t.Errorf("foldText = %q", got)
	}
	for _, r := range foldText("Zoë Ångström-O'Brien, Jr. #42") {
		if _, ok := font[r]; !ok && r != ' ' {
			t.Errorf("font lacks %q", r)
		}
	}
}

func TestRoundRectWalkIsContinuous(t *testing.T) {
	rr := roundRect{0, 0, CardWidth, CardHeight, cornerR}.inset(2.6)
	prev, _ := rr.at(0)
	steps := 2000
	for i := 1; i <= steps; i++ {
		p, _ := rr.at(rr.perimeter() * float64(i) / float64(steps))
		if d := dist(p, prev); d > rr.perimeter()/float64(steps)*1.01 {
			t.Fatalf("walk jumps %.3fmm at step %d", d, i)
		}
		prev = p
	}
}

func TestRenderStaysOnTheCard(t *testing.T) {
	checkOnCard(t, Render(sampleCard()), "pattern", "text", "qr", "cut")
	if path := os.Getenv("BADGE_PREVIEW"); path != "" {
		if err := os.WriteFile(path, Render(sampleCard()), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func TestRenderBackStaysOnTheCard(t *testing.T) {
	iss, _ := Issuer("secret", "https://multipass.sequoia.garden")
	rec := Record{Serial: 17, MemberID: "42", Name: "Alexandra Montgomery-Smith", IssuedAt: time.Now(), StatusIndex: 99}
	text, err := BarcodeText(iss, "https://multipass.sequoia.garden", rec, Sign(17, "42", "secret"))
	if err != nil {
		t.Fatal(err)
	}
	pdf, mod, err := pdf417.Fit(text, 5, BarcodeBox.Height, BarcodeBox.Width)
	if err != nil {
		t.Fatal(err)
	}
	if mod < 0.25 {
		t.Errorf("modules %.3fmm: too fine to burn", mod)
	}
	t.Logf("%d chars, %d rows of %d modules, %.3fmm", len(text), len(pdf), len(pdf[0]), mod)
	back := sampleBack(pdf)
	checkOnCard(t, RenderBack(back), "align", "pattern", "text", "pdf417")
	// Long and short symbols are limited by each side of the box in turn
	checkOnCard(t, RenderBack(sampleBack(fakeMatrix(8, 290, 3))), "pdf417")
	checkOnCard(t, RenderBack(sampleBack(fakeMatrix(90, 120, 4))), "pdf417")
	if path := os.Getenv("BADGE_PREVIEW_BACK"); path != "" {
		if err := os.WriteFile(path, RenderBack(back), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func checkOnCard(t *testing.T, svg []byte, layers ...string) {
	t.Helper()

	dec := xml.NewDecoder(strings.NewReader(string(svg)))
	groups := map[string]bool{}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("SVG is not well-formed: %v", err)
		}
		el, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range el.Attr {
			if el.Name.Local == "g" && a.Name.Local == "id" {
				groups[a.Value] = true
			}
			if el.Name.Local != "path" || a.Name.Local != "d" {
				continue
			}
			// Absolute M/L coordinates come in x,y pairs; the QR's relative
			// h/v steps are checked by its own bounds below.
			for _, pair := range regexp.MustCompile(`M?(-?[\d.]+),(-?[\d.]+)`).FindAllStringSubmatch(a.Value, -1) {
				x, _ := strconv.ParseFloat(pair[1], 64)
				y, _ := strconv.ParseFloat(pair[2], 64)
				if x < 0 || x > CardWidth || y < 0 || y > CardHeight {
					t.Errorf("point (%.3f, %.3f) is off the card", x, y)
				}
			}
		}
	}
	for _, id := range layers {
		if !groups[id] {
			t.Errorf("missing layer %q", id)
		}
	}
}

func TestRosetteIsPerMember(t *testing.T) {
	a := sampleCard()
	b := sampleCard()
	b.Sig = Sign(18, "43", "secret")
	if string(Render(a)) == string(Render(b)) {
		t.Error("two members got identical badges")
	}
}

func sampleCard() Card {
	sig := Sign(17, "42", "secret")
	u, _ := URL("https://multipass.sequoia.garden", 17, sig)
	return Card{
		Org:      "Sequoia Fabrica",
		Name:     "Ada Lovelace-Byron",
		MemberID: "42",
		Serial:   17,
		Since:    "2024",
		Issued:   time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC),
		URL:      u,
		Sig:      sig,
		QR:       fakeQR(33),
		Slot:     true,
	}
}

func sampleBack(pdf [][]bool) Back {
	return Back{
		Org:    "Sequoia Fabrica",
		Host:   "multipass.sequoia.garden",
		Issuer: "did:web:multipass.sequoia.garden",
		Serial: 17,
		PDF417: pdf,
	}
}

// fakeMatrix stands in for a PDF417: noise, rows by columns.
func fakeMatrix(rows, cols int, seed uint64) [][]bool {
	rng := rand.New(rand.NewPCG(seed, seed))
	m := make([][]bool, rows)
	for r := range m {
		m[r] = make([]bool, cols)
		for c := range m[r] {
			m[r][c] = rng.IntN(2) == 0
		}
	}
	return m
}

// fakeQR stands in for a real QR matrix: finder patterns and noise. The real
// encoder lives outside this package.
func fakeQR(n int) [][]bool {
	rng := rand.New(rand.NewPCG(1, 2))
	m := make([][]bool, n)
	for r := range m {
		m[r] = make([]bool, n)
		for c := range m[r] {
			m[r][c] = rng.IntN(2) == 0
		}
	}
	for _, o := range [][2]int{{0, 0}, {0, n - 7}, {n - 7, 0}} {
		for r := -1; r <= 7; r++ {
			for c := -1; c <= 7; c++ {
				rr, cc := o[0]+r, o[1]+c
				if rr < 0 || cc < 0 || rr >= n || cc >= n {
					continue
				}
				ring := max(abs(r-3), abs(c-3))
				m[rr][cc] = ring != 2 && ring != 4
			}
		}
	}
	return m
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

func dist(a, b pt) float64 {
	return math.Hypot(a.X-b.X, a.Y-b.Y)
}
