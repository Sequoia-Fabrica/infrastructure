package utils

import (
	"crypto/md5"
	"fmt"
	"strings"
)

// GenerateGravatarURL creates a Gravatar URL from an email address
// Size parameter specifies the size of the image in pixels (1-2048)
// Default parameter specifies the fallback image style:
// - 404: Return 404 error if no image found
// - mp: Mystery Person silhouette
// - identicon: Geometric pattern
// - monsterid: Generated monster
// - wavatar: Generated face
// - retro: 8-bit arcade style
// - robohash: Robot
// - blank: Transparent PNG
func GenerateGravatarURL(email string, size int, defaultImage string) string {
	// Trim whitespace and convert to lowercase
	email = strings.TrimSpace(strings.ToLower(email))

	// Generate MD5 hash of email
	hash := md5.Sum([]byte(email))
	hashString := fmt.Sprintf("%x", hash)

	// Build Gravatar URL
	return fmt.Sprintf("https://www.gravatar.com/avatar/%s?s=%d&d=%s",
		hashString, size, defaultImage)
}
