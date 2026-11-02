package handlers

import (
	"encoding/json"
	"errors"
	"fmt"
	"multipass/internal/badge"
	"multipass/internal/config"
	"multipass/internal/models"
	"multipass/internal/pdf417"
	"multipass/internal/services"
	"multipass/internal/utils"
	"multipass/internal/vcb"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/skip2/go-qrcode"
)

// BadgeHandlers serve the wooden badge pages: making a badge's laser files,
// and reporting a laser job failed or a badge lost. Making the files records
// the badge as lasered, and from then its QR code opens the member's card.
type BadgeHandlers struct {
	Store       *badge.Store
	statusLists statusListCache
}

// Page lists a member's badges and offers to make a new one. Members see
// their own; staff can open anyone's with ?member=<id>.
func (h *BadgeHandlers) Page(c *gin.Context) {
	cfg := config.Load()
	member, status, err := h.member(c, cfg, c.Query("member"))
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}

	badges := h.Store.ForMember(member.MemberID)
	var active *badge.Record
	for i := range badges {
		if badges[i].Active() {
			active = &badges[i]
		}
	}
	c.HTML(http.StatusOK, "badge.html", gin.H{
		"title":           "Wooden Badge - " + cfg.MakerspaceName,
		"makerspace_name": cfg.MakerspaceName,
		"user":            c.MustGet("user"),
		"member":          member,
		"badges":          badges,
		"active":          active,
		"just_made":       c.Query("made"),
		"error":           c.Query("error"),
	})
}

// Preview draws a badge as it would come out, without issuing one. Its codes
// are placeholders that open nothing.
func (h *BadgeHandlers) Preview(c *gin.Context) {
	cfg := config.Load()
	member, status, err := h.member(c, cfg, c.Query("member"))
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	h.writeFront(c, cfg, member, badge.Record{MemberID: member.MemberID, Name: member.FullName, IssuedAt: time.Now()}, "PREVIEW")
}

// Issue records a new badge for the member and sends them back to the page
// to download its files. A member holds one active badge at a time.
func (h *BadgeHandlers) Issue(c *gin.Context) {
	cfg := config.Load()
	member, status, err := h.member(c, cfg, c.PostForm("member"))
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	user := c.MustGet("user").(*models.UserProfile)

	rec, err := h.Store.Issue(member.MemberID, member.FullName, user.Email, time.Now())
	if err != nil {
		h.backToPage(c, member.MemberID, "error", err.Error())
		return
	}
	services.NewLogger(cfg).Info("Badge %d issued to member %s by %s", rec.Serial, rec.MemberID, user.Email)
	h.backToPage(c, member.MemberID, "made", strconv.Itoa(rec.Serial))
}

// Void marks a badge failed (the laser job went wrong) or lost. Either way its
// QR code stops opening the member's card, for good.
func (h *BadgeHandlers) Void(status badge.Status) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		rec, code, err := h.record(c, cfg)
		if err != nil {
			c.JSON(code, gin.H{"error": err.Error()})
			return
		}
		user := c.MustGet("user").(*models.UserProfile)

		if _, err := h.Store.Void(rec.Serial, status, user.Email, time.Now()); err != nil {
			h.backToPage(c, rec.MemberID, "error", err.Error())
			return
		}
		services.NewLogger(cfg).Info("Badge %d of member %s marked %s by %s", rec.Serial, rec.MemberID, status, user.Email)
		h.backToPage(c, rec.MemberID, "", "")
	}
}

// Front returns the laser file for the front of an active badge. ?download=1
// saves it as a file; ?slot=0 leaves out the lanyard slot.
func (h *BadgeHandlers) Front(c *gin.Context) {
	cfg := config.Load()
	rec, code, err := h.record(c, cfg)
	if err != nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	member, err := services.NewAuthentikClient(cfg).GetUserByID(rec.MemberID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "member not found"})
		return
	}
	h.writeFront(c, cfg, member, rec, badge.Sign(rec.Serial, rec.MemberID, cfg.BadgeSecret))
}

// Back returns the laser file for the back of an active badge: a return
// address and the badge's signed AAMVA PDF417.
func (h *BadgeHandlers) Back(c *gin.Context) {
	cfg := config.Load()
	rec, code, err := h.record(c, cfg)
	if err != nil {
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}
	iss, err := badge.Issuer(cfg.BadgeSecret, cfg.PublicURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	h.writeBack(c, cfg, iss, rec, badge.Sign(rec.Serial, rec.MemberID, cfg.BadgeSecret))
}

// PreviewBack draws the back of a badge as it would come out, without
// issuing one. Its barcode is signed with a throwaway key, so it is the
// right size but verifies as nothing.
func (h *BadgeHandlers) PreviewBack(c *gin.Context) {
	cfg := config.Load()
	member, status, err := h.member(c, cfg, c.Query("member"))
	if err != nil {
		c.JSON(status, gin.H{"error": err.Error()})
		return
	}
	iss, err := badge.Issuer(cfg.BadgeSecret, cfg.PublicURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	iss.Key = vcb.GenerateKey()
	rec := badge.Record{MemberID: member.MemberID, Name: member.FullName, IssuedAt: time.Now(), StatusIndex: vcb.RandomStatusIndex()}
	h.writeBack(c, cfg, iss, rec, "PREVIEW")
}

func (h *BadgeHandlers) writeBack(c *gin.Context, cfg *config.Config, iss vcb.Issuer, rec badge.Record, sig string) {
	host, err := badge.Host(cfg.PublicURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	text, err := badge.BarcodeText(iss, cfg.PublicURL, rec, sig)
	if err != nil {
		services.NewLogger(cfg).Error("Badge %d: signing barcode: %v", rec.Serial, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sign the barcode"})
		return
	}
	// Level 5 spends 64 codewords on error correction, AAMVA's recommended
	// margin, and on wood a welcome one
	box := badge.BarcodeBox
	pdf, _, err := pdf417.Fit(text, 5, box.Height, box.Width)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate PDF417"})
		return
	}

	svg := badge.RenderBack(badge.Back{
		Org:    cfg.MakerspaceName,
		Host:   host,
		Issuer: iss.DID,
		Serial: rec.Serial,
		PDF417: pdf,
	})
	h.sendSVG(c, svg, fmt.Sprintf("badge-%d-back.svg", rec.Serial))
}

// DIDDocument serves the makerspace's did:web document, with the key badge
// barcodes and status lists are signed with.
func DIDDocument(c *gin.Context) {
	cfg := config.Load()
	if cfg.BadgeSecret == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "badges are not configured"})
		return
	}
	iss, err := badge.Issuer(cfg.BadgeSecret, cfg.PublicURL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Header("Cache-Control", "public, max-age=3600")
	c.JSON(http.StatusOK, iss.DIDDocument())
}

// StatusList serves one of the badge revocation lists. A verifier holding a
// badge barcode finds its list here and sees whether the badge was voided.
func (h *BadgeHandlers) StatusList(c *gin.Context) {
	cfg := config.Load()
	list, err := strconv.Atoi(c.Param("list"))
	if err != nil || list < 0 || list >= vcb.ListCount || cfg.BadgeSecret == "" {
		c.JSON(http.StatusNotFound, gin.H{"error": "no such status list"})
		return
	}
	body, err := h.statusLists.get(h.Store, cfg, list)
	if err != nil {
		services.NewLogger(cfg).Error("Badge status list %d: %v", list, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to make the status list"})
		return
	}
	// Verifiers may cache it this long; a badge marked lost shows up in
	// lists fetched after that
	c.Header("Cache-Control", "public, max-age=300")
	c.Data(http.StatusOK, "application/vc+ld+json", body)
}

// statusListCache keeps each signed list until the store changes: a list is
// 8 MiB of bits before compression, too much to rebuild on every fetch.
type statusListCache struct {
	mu      sync.Mutex
	version int
	lists   map[int][]byte
}

func (s *statusListCache) get(store *badge.Store, cfg *config.Config, list int) ([]byte, error) {
	voided, version := store.Voided()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lists == nil || s.version != version {
		s.lists, s.version = map[int][]byte{}, version
	}
	if body, ok := s.lists[list]; ok {
		return body, nil
	}
	iss, err := badge.Issuer(cfg.BadgeSecret, cfg.PublicURL)
	if err != nil {
		return nil, err
	}
	vc, err := iss.StatusListCredential(badge.StatusBaseURL(cfg.PublicURL), "revocation", list, voided, time.Now())
	if err != nil {
		return nil, err
	}
	body, err := json.Marshal(vc)
	if err != nil {
		return nil, err
	}
	s.lists[list] = body
	return body, nil
}

func (h *BadgeHandlers) writeFront(c *gin.Context, cfg *config.Config, member *models.UserProfile, rec badge.Record, sig string) {
	badgeURL, err := badge.URL(cfg.PublicURL, rec.Serial, sig)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	// High (25%) error correction: wood grain and uneven burns cost modules
	qr, err := utils.GenerateQRMatrix(badgeURL, qrcode.High)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate QR code"})
		return
	}

	svg := badge.Render(badge.Card{
		Org:      cfg.MakerspaceName,
		Name:     rec.Name,
		MemberID: rec.MemberID,
		Serial:   rec.Serial,
		Since:    sinceYear(member.MemberSince),
		Issued:   rec.IssuedAt,
		URL:      badgeURL,
		Sig:      sig,
		QR:       qr,
		Slot:     c.Query("slot") != "0",
	})
	h.sendSVG(c, svg, fmt.Sprintf("badge-%d-front-%s.svg", rec.Serial, slug(rec.Name)))
}

func (h *BadgeHandlers) sendSVG(c *gin.Context, svg []byte, filename string) {
	if c.Query("download") != "" {
		c.Header("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, filename))
	}
	c.Header("Cache-Control", "no-store")
	c.Data(http.StatusOK, "image/svg+xml", svg)
}

func (h *BadgeHandlers) backToPage(c *gin.Context, memberID, key, value string) {
	q := url.Values{"member": {memberID}}
	if key != "" {
		q.Set(key, value)
	}
	c.Redirect(http.StatusSeeOther, "/badge?"+q.Encode())
}

// member resolves whose badges are being worked on, defaulting to the
// signed-in user, and fetches their Authentik record.
func (h *BadgeHandlers) member(c *gin.Context, cfg *config.Config, memberID string) (*models.UserProfile, int, error) {
	if cfg.BadgeSecret == "" {
		return nil, http.StatusServiceUnavailable, errors.New("badges are not configured: BADGE_SECRET is unset")
	}
	user := c.MustGet("user").(*models.UserProfile)
	if memberID == "" {
		memberID = user.MemberID
	}
	if !mayManage(user, memberID) {
		return nil, http.StatusForbidden, errors.New("only staff can manage other members' badges")
	}
	if !badge.IsMemberID(memberID) {
		return nil, http.StatusBadRequest, errors.New("no member ID to make a badge for")
	}
	member, err := services.NewAuthentikClient(cfg).GetUserByID(memberID)
	if err != nil {
		return nil, http.StatusNotFound, fmt.Errorf("member %s not found", memberID)
	}
	return member, http.StatusOK, nil
}

// record resolves the badge in the URL, which must be active and belong to
// the signed-in user unless they are staff.
func (h *BadgeHandlers) record(c *gin.Context, cfg *config.Config) (badge.Record, int, error) {
	if cfg.BadgeSecret == "" {
		return badge.Record{}, http.StatusServiceUnavailable, errors.New("badges are not configured: BADGE_SECRET is unset")
	}
	serial, err := strconv.Atoi(c.Param("serial"))
	if err != nil {
		return badge.Record{}, http.StatusNotFound, badge.ErrNoBadge
	}
	rec, ok := h.Store.Get(serial)
	if !ok || !mayManage(c.MustGet("user").(*models.UserProfile), rec.MemberID) {
		return badge.Record{}, http.StatusNotFound, badge.ErrNoBadge
	}
	if !rec.Active() {
		return badge.Record{}, http.StatusGone, badge.ErrNotActive
	}
	return rec, http.StatusOK, nil
}

func mayManage(user *models.UserProfile, memberID string) bool {
	return memberID == user.MemberID || user.AccessLevel >= models.Staff
}

// sinceYear pulls the year out of a member_since attribute like "2024-03-01"
func sinceYear(memberSince string) string {
	return regexp.MustCompile(`\b(19|20)\d\d\b`).FindString(memberSince)
}

func slug(name string) string {
	s := strings.Trim(regexp.MustCompile(`[^a-z0-9]+`).ReplaceAllString(strings.ToLower(name), "-"), "-")
	if s == "" {
		return "member"
	}
	return s
}
