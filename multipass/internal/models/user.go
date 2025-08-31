package models

import (
	"strings"
)

type UserLevel int

const (
	LimitedVolunteer UserLevel = iota
	FullMember
	Staff
	Admin
)

func (ul UserLevel) String() string {
	switch ul {
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
	Email       string    `json:"email"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	Groups      []string  `json:"groups"`
	Avatar      *string   `json:"avatar,omitempty"`
	Phone       *string   `json:"phone,omitempty"`
	MemberID    string    `json:"member_id"`
	AccessLevel UserLevel `json:"access_level"`
}

type UserFromHeaders struct {
	Email     string   `json:"email"`
	FirstName string   `json:"first_name"`
	LastName  string   `json:"last_name"`
	Username  string   `json:"username"`
	UserID    string   `json:"user_id"`
	Groups    []string `json:"groups"`
}

// Group mapping for membership types and user levels
var GroupMapping = map[string]UserLevel{
	"volunteers-limited": LimitedVolunteer,
	"members-full":       FullMember,
}

// DetermineUserLevel determines user level from Authentik groups
func DetermineUserLevel(groups []string) UserLevel {
	// Check for highest privilege level first
	for _, group := range groups {
		if level, exists := GroupMapping[group]; exists {
			return level
		}
	}
	return LimitedVolunteer // Default to limited volunteer
}

// GetFullName returns the user's full name
func (u *UserProfile) GetFullName() string {
	return strings.TrimSpace(u.FirstName + " " + u.LastName)
}

// GetInitials returns the user's initials
func (u *UserProfile) GetInitials() string {
	initials := ""
	if len(u.FirstName) > 0 {
		initials += string(u.FirstName[0])
	}
	if len(u.LastName) > 0 {
		initials += string(u.LastName[0])
	}
	return strings.ToUpper(initials)
}
