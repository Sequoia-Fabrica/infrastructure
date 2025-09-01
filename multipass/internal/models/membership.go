package models

import "time"

type MembershipStatus int

const (
	StatusInactive MembershipStatus = iota
	StatusActive
	StatusSuspended
	StatusExpired
)

func (ms MembershipStatus) String() string {
	switch ms {
	case StatusInactive:
		return "Inactive"
	case StatusActive:
		return "Active"
	case StatusSuspended:
		return "Suspended"
	case StatusExpired:
		return "Expired"
	default:
		return "Unknown"
	}
}

type MembershipInfo struct {
	MembershipType    string           `json:"membership_type"`
	Status            MembershipStatus `json:"status"`
	UserLevel         UserLevel        `json:"user_level"`
	JoinDate          *time.Time       `json:"join_date,omitempty"`
	ExpiryDate        *time.Time       `json:"expiry_date,omitempty"`
}

// IsActive returns true if the membership is currently active
func (m *MembershipInfo) IsActive() bool {
	return m.Status == StatusActive
}

// GetAccessLevel returns a human-readable access level description
func (m *MembershipInfo) GetAccessLevel() string {
	switch m.UserLevel {
	case NoAccess:
		return "No access to workspace"
	case LimitedVolunteer:
		return "Basic workspace access with supervision"
	case FullMember:
		return "Full workspace access"
	case Staff:
		return "Staff privileges and administrative access"
	case Admin:
		return "Full administrative access"
	default:
		return "Unknown access level"
	}
}
