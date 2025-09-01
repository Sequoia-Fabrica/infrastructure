package models

import (
	"multipass/internal/config"
	"strings"
)

type UserLevel int

const (
	NoAccess UserLevel = iota
	LimitedVolunteer
	FullMember
	Staff
	Admin
)

func (ul UserLevel) String() string {
	switch ul {
	case NoAccess:
		return "No Access"
	case LimitedVolunteer:
		return "Limited Volunteer"
	case FullMember:
		return "Full Member"
	case Staff:
		return "Staff"
	case Admin:
		return "Admin"
	default:
		return "Unknown"
	}
}

type UserProfile struct {
	Email            string    `json:"email"`
	FullName         string    `json:"full_name"`
	Groups           []string  `json:"groups"`
	Avatar           *string   `json:"avatar,omitempty"`
	Phone            *string   `json:"phone,omitempty"`
	MemberID         string    `json:"member_id"`
	AccessLevel      UserLevel `json:"access_level"`
	AuthentikID      string    `json:"authentik_id,omitempty"`
	MemberSince      string    `json:"member_since,omitempty"`
	MembershipType   string    `json:"membership_type,omitempty"`
	ExpiryDate       string    `json:"expiry_date,omitempty"`
	MembershipStatus string    `json:"membership_status,omitempty"`
}

type UserFromHeaders struct {
	Email    string   `json:"email"`
	FullName string   `json:"full_name"`
	Username string   `json:"username"`
	UserID   string   `json:"user_id"`
	Groups   []string `json:"groups"`
}

// Group mapping will be loaded from config file
var GroupMapping = map[string]UserLevel{}

// DetermineUserLevel determines user level from Authentik groups
func DetermineUserLevel(groups []string) UserLevel {
	// Import config package
	cfg := config.Load()

	// Check for highest privilege level first
	for _, group := range groups {
		if levelStr, exists := cfg.GroupMappingConfig.Mappings[group]; exists {
			// Convert string level to UserLevel enum
			switch levelStr {
			case "NoAccess":
				return NoAccess
			case "LimitedVolunteer":
				return LimitedVolunteer
			case "FullMember":
				return FullMember
			case "Staff":
				return Staff
			case "Admin":
				return Admin
			default:
				return NoAccess
			}
		}
	}

	// Use default level from config
	switch cfg.GroupMappingConfig.DefaultLevel {
	case "NoAccess":
		return NoAccess
	case "LimitedVolunteer":
		return LimitedVolunteer
	case "FullMember":
		return FullMember
	case "Staff":
		return Staff
	case "Admin":
		return Admin
	default:
		return NoAccess
	}
}

// GetFullName returns the user's full name
func (u *UserProfile) GetFullName() string {
	return u.FullName
}

// GetInitials returns the user's initials
func (u *UserProfile) GetInitials() string {
	parts := strings.Fields(u.FullName)
	initials := ""

	// Get first letter of first name
	if len(parts) > 0 && len(parts[0]) > 0 {
		initials += string(parts[0][0])
	}

	// Get first letter of last name (if available)
	if len(parts) > 1 && len(parts[len(parts)-1]) > 0 {
		initials += string(parts[len(parts)-1][0])
	}

	return strings.ToUpper(initials)
}
