package main

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"multipass/internal/badge"
	"multipass/internal/config"
	"multipass/internal/fakeauthentik"
	"multipass/internal/services"

	"github.com/gin-gonic/gin"
)

// Badge actions go through the same router as in production, against a
// fake Authentik, to check who may do what.

type who struct{ email, name, groups string }

var (
	anonymous = who{}
	member42  = who{"test.user@example.com", "Test User", "Members"}
	staff     = who{"staff@example.com", "Sam Staff", "Members|staff"}
)

func setup(t *testing.T) (*gin.Engine, *badge.Store) {
	t.Helper()
	// Templates and the group mapping are found relative to the module root
	wd, _ := os.Getwd()
	if err := os.Chdir(filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chdir(wd) })

	authentik := httptest.NewServer(fakeauthentik.Handler())
	t.Cleanup(authentik.Close)
	t.Setenv("ENVIRONMENT", "production") // no development sign-in
	t.Setenv("AUTHENTIK_URL", authentik.URL)
	t.Setenv("AUTHENTIK_API_TOKEN", "test")
	t.Setenv("TOKEN_SECRET", "test")
	t.Setenv("BADGE_SECRET", "test-badge-secret")
	t.Setenv("PUBLIC_URL", "https://multipass.example.org")
	t.Setenv("GROUP_MAPPING_CONFIG", "./config/group_mapping.yaml")
	gin.SetMode(gin.TestMode)

	cfg := config.Load()
	store, err := badge.OpenStore(filepath.Join(t.TempDir(), "badges.json"))
	if err != nil {
		t.Fatal(err)
	}
	return newRouter(cfg, services.NewLogger(cfg), store), store
}

// do makes a request as someone, from multipass's own pages unless site says
// otherwise.
func do(r *gin.Engine, as who, method, path string, form url.Values, site string) *httptest.ResponseRecorder {
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	req := httptest.NewRequest(method, path, body)
	if form != nil {
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	}
	if site == "" {
		site = "same-origin"
	}
	req.Header.Set("Sec-Fetch-Site", site)
	if as.email != "" {
		req.Header.Set("X-Authentik-Email", as.email)
		req.Header.Set("X-Authentik-Name", as.name)
		req.Header.Set("X-Authentik-Groups", as.groups)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func status(t *testing.T, store *badge.Store, serial int) badge.Status {
	t.Helper()
	rec, ok := store.Get(serial)
	if !ok {
		t.Fatalf("badge %d missing", serial)
	}
	return rec.Status
}

func TestBadgeActionsNeedSignIn(t *testing.T) {
	r, store := setup(t)
	if _, err := store.Issue("43", "Ada", "ada@example.com", time.Now()); err != nil {
		t.Fatal(err)
	}

	for _, req := range []struct{ method, path string }{
		{"GET", "/badge"},
		{"POST", "/badge"},
		{"GET", "/badge/preview.svg?member=43"},
		{"GET", "/badge/preview-back.svg?member=43"},
		{"GET", "/badges/1/front.svg"},
		{"GET", "/badges/1/back.svg"},
		{"POST", "/badges/1/failed"},
		{"POST", "/badges/1/lost"},
	} {
		form := url.Values{"member": {"43"}}
		if w := do(r, anonymous, req.method, req.path, form, ""); w.Code != http.StatusUnauthorized {
			t.Errorf("anonymous %s %s: %d, want 401", req.method, req.path, w.Code)
		}
	}
	if status(t, store, 1) != badge.Lasered || len(store.ForMember("43")) != 1 {
		t.Error("an anonymous request changed the store")
	}

	// What a scanner or verifier needs stays public
	for _, path := range []string{"/.well-known/did.json", "/badge-status/revocation/0"} {
		if w := do(r, anonymous, "GET", path, nil, "none"); w.Code != http.StatusOK {
			t.Errorf("anonymous GET %s: %d, want 200", path, w.Code)
		}
	}
}

func TestMembersActOnlyOnTheirOwnBadges(t *testing.T) {
	r, store := setup(t)
	if _, err := store.Issue("43", "Ada", "ada@example.com", time.Now()); err != nil {
		t.Fatal(err)
	}

	// Someone else's badge: invisible, and can't be voided
	if w := do(r, member42, "GET", "/badges/1/front.svg", nil, ""); w.Code != http.StatusNotFound {
		t.Errorf("member 42 downloading 43's badge: %d, want 404", w.Code)
	}
	for _, path := range []string{"/badges/1/lost", "/badges/1/failed"} {
		if w := do(r, member42, "POST", path, nil, ""); w.Code != http.StatusNotFound {
			t.Errorf("member 42 POST %s on 43's badge: %d, want 404", path, w.Code)
		}
	}
	if w := do(r, member42, "POST", "/badge", url.Values{"member": {"43"}}, ""); w.Code != http.StatusForbidden {
		t.Errorf("member 42 issuing 43 a badge: %d, want 403", w.Code)
	}
	if status(t, store, 1) != badge.Lasered || len(store.ForMember("43")) != 1 {
		t.Fatal("member 42 changed member 43's badges")
	}

	// Their own: make it, download it, report it
	w := do(r, member42, "POST", "/badge", url.Values{"member": {"42"}}, "")
	if w.Code != http.StatusSeeOther || !strings.Contains(w.Header().Get("Location"), "made=2") {
		t.Fatalf("member 42 making a badge: %d %s", w.Code, w.Header().Get("Location"))
	}
	if w := do(r, member42, "GET", "/badges/2/back.svg", nil, ""); w.Code != http.StatusOK {
		t.Errorf("member 42 downloading their badge: %d", w.Code)
	}
	if w := do(r, member42, "POST", "/badges/2/lost", nil, "cross-site"); w.Code != http.StatusForbidden {
		t.Errorf("cross-site post: %d, want 403", w.Code)
	}
	if status(t, store, 2) != badge.Lasered {
		t.Fatal("a cross-site post voided a badge")
	}
	if w := do(r, member42, "POST", "/badges/2/lost", nil, ""); w.Code != http.StatusSeeOther {
		t.Errorf("member 42 reporting their badge lost: %d", w.Code)
	}
	if rec, _ := store.Get(2); rec.Status != badge.Lost || rec.VoidedBy != member42.email {
		t.Errorf("badge 2 = %+v, want lost by member 42", rec)
	}
}

func TestStaffActOnAnyonesBadges(t *testing.T) {
	r, store := setup(t)
	if _, err := store.Issue("43", "Ada", "ada@example.com", time.Now()); err != nil {
		t.Fatal(err)
	}
	if w := do(r, staff, "GET", "/badges/1/front.svg", nil, ""); w.Code != http.StatusOK {
		t.Errorf("staff downloading member 43's badge: %d", w.Code)
	}
	if w := do(r, staff, "POST", "/badges/1/failed", nil, ""); w.Code != http.StatusSeeOther {
		t.Errorf("staff voiding member 43's badge: %d", w.Code)
	}
	if rec, _ := store.Get(1); rec.Status != badge.Failed || rec.VoidedBy != staff.email {
		t.Errorf("badge 1 = %+v, want failed by staff", rec)
	}
	if w := do(r, staff, "POST", "/badge", url.Values{"member": {"43"}}, ""); w.Code != http.StatusSeeOther {
		t.Errorf("staff issuing member 43 a badge: %d", w.Code)
	}
	if got := store.ForMember("43"); len(got) != 2 || got[0].IssuedBy != staff.email {
		t.Errorf("member 43's badges = %+v", got)
	}
}

func TestCardLinksOwnerToBadges(t *testing.T) {
	r, _ := setup(t)
	w := do(r, member42, "GET", "/card", nil, "")
	loc := w.Header().Get("Location")
	if w.Code != http.StatusTemporaryRedirect || !strings.HasPrefix(loc, "/public/card?token=") {
		t.Fatalf("/card: %d %s", w.Code, loc)
	}
	var owner *http.Cookie
	for _, c := range w.Result().Cookies() {
		if c.Name == "multipass_card_owner" {
			owner = c
		}
	}
	if owner == nil || owner.Value != "42" || !owner.HttpOnly {
		t.Fatalf("/card set owner cookie %+v", owner)
	}

	view := func(cookie *http.Cookie) bool {
		req := httptest.NewRequest("GET", loc, nil)
		if cookie != nil {
			req.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("card page: %d", w.Code)
		}
		return strings.Contains(w.Body.String(), `href="/badge"`)
	}
	// The owner sees the link, reload after reload
	for i := 0; i < 2; i++ {
		if !view(owner) {
			t.Error("owner's card has no link to /badge")
		}
	}
	// Anyone else looking at the card doesn't
	if view(nil) {
		t.Error("a stranger's view links to /badge")
	}
	if view(&http.Cookie{Name: "multipass_card_owner", Value: "43"}) {
		t.Error("another member's view links to /badge")
	}
}
