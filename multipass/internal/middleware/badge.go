package middleware

import (
	"multipass/internal/badge"
	"multipass/internal/config"
	"multipass/internal/services"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gin-gonic/gin"
)

// BadgeAuthMiddleware checks the code on a scanned wooden badge and sets the
// member it belongs to in the context. The code never expires; it stops
// working when the badge is marked failed or lost.
func BadgeAuthMiddleware(store *badge.Store) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.Load()
		logger := services.NewLogger(cfg)

		reject := func(message string) {
			c.HTML(http.StatusNotFound, "login.html", gin.H{
				"title":           "Badge Not Valid - " + cfg.MakerspaceName,
				"makerspace_name": cfg.MakerspaceName,
				"error":           message,
			})
			c.Abort()
		}
		// Unknown badges and bad signatures get the same answer, so the
		// route can't be used to probe which serials exist
		const notValid = "This badge isn't valid."

		serial, err := strconv.Atoi(c.Param("serial"))
		if err != nil || cfg.BadgeSecret == "" {
			reject(notValid)
			return
		}
		rec, ok := store.Get(serial)
		if !ok || !badge.Verify(rec.Serial, rec.MemberID, c.Param("sig"), cfg.BadgeSecret) {
			reject(notValid)
			return
		}

		// A genuine badge that has been voided: someone holding one marked
		// lost may well be trying to return it
		switch rec.Status {
		case badge.Lost:
			logger.Info("Scanned badge %d, reported lost", rec.Serial)
			reject("This badge was reported lost and no longer works. If you found it, please return it to " + cfg.MakerspaceName + ".")
			return
		case badge.Failed:
			reject(notValid)
			return
		}

		member, err := services.NewAuthentikClient(cfg).GetUserByID(rec.MemberID)
		if err != nil {
			logger.Error("Badge %d: member %s lookup failed: %v", rec.Serial, rec.MemberID, err)
			reject(notValid)
			return
		}

		c.Set("user", member)
		c.Set("badge_view", true)
		c.Next()
	}
}

// SameOriginMiddleware refuses form posts from other sites. Authentik's
// session cookie rides along on any request to multipass, so without this a
// page elsewhere could post a form that marks someone's badge lost.
func SameOriginMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			c.Next()
			return
		}
		// Browsers send Sec-Fetch-Site on every request; fall back to Origin
		switch c.GetHeader("Sec-Fetch-Site") {
		case "same-origin", "none":
			c.Next()
			return
		case "":
			if o, err := url.Parse(c.GetHeader("Origin")); err == nil && o.Host == c.Request.Host {
				c.Next()
				return
			}
		}
		c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "cross-site request refused"})
	}
}
