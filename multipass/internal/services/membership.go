package services

import (
	"multipass/internal/config"
	"multipass/internal/models"
	"strings"
	"time"
)

// MembershipService provides methods to retrieve membership information
type MembershipService struct {
	cfg            *config.Config
	authentikClient *AuthentikClient
	logger         *Logger
}

// NewMembershipService creates a new instance of MembershipService
func NewMembershipService() *MembershipService {
	cfg := config.Load()
	authentikClient := NewAuthentikClient(cfg)
	logger := NewLogger(cfg)

	return &MembershipService{
		cfg:            cfg,
		authentikClient: authentikClient,
		logger:         logger,
	}
}

// GetMembershipInfo retrieves membership information for a user
// This implementation uses Authentik data and group mappings to determine membership details
func (s *MembershipService) GetMembershipInfo(user *models.UserProfile) (*models.MembershipInfo, error) {
	// Log that we're retrieving membership info
	s.logger.Debug("Retrieving membership info for user: %s", user.Email)

	// If we have an Authentik ID, try to refresh user data from Authentik
	var refreshedUser *models.UserProfile
	var err error

	if user.AuthentikID != "" {
		refreshedUser, err = s.authentikClient.GetUserByID(user.AuthentikID)
		if err != nil {
			s.logger.Debug("Failed to refresh user data from Authentik by ID: %v", err)
			// Continue with the user data we have
			refreshedUser = user
		}
	} else if user.Email != "" {
		// Try to get user by email if we don't have an Authentik ID
		refreshedUser, err = s.authentikClient.GetUserByEmail(user.Email)
		if err != nil {
			s.logger.Debug("Failed to refresh user data from Authentik by email: %v", err)
			// Continue with the user data we have
			refreshedUser = user
		}
	} else {
		// No identifiers available, use the user data we have
		refreshedUser = user
	}

	// Determine membership type based on metadata or access level
	membershipType := s.getMembershipType(refreshedUser)

	// Determine membership status based on Authentik groups and access level
	status := s.determineMembershipStatus(refreshedUser)

	// Get join date and expiry date
	joinDate := s.getJoinDate(refreshedUser)
	expiryDate := s.getExpiryDate(refreshedUser)

	// Create and return the membership info
	membershipInfo := &models.MembershipInfo{
		MembershipType: membershipType,
		Status:         status,
		UserLevel:      refreshedUser.AccessLevel,
		JoinDate:       joinDate,
		ExpiryDate:     expiryDate,
	}

	return membershipInfo, nil
}

// getMembershipType returns a human-readable membership type based on metadata or access level
func (s *MembershipService) getMembershipType(user *models.UserProfile) string {
	// Check if we have membership_type metadata from Authentik
	if user.MembershipType != "" {
		s.logger.Debug("Using membership_type metadata: %s for user %s", user.MembershipType, user.Email)
		return user.MembershipType
	}
	
	// Fall back to access level mapping if no metadata
	switch user.AccessLevel {
	case models.NoAccess:
		return "No Access"
	case models.LimitedVolunteer:
		return "Limited Volunteer"
	case models.FullMember:
		return "Full Member"
	case models.Staff:
		return "Staff Member"
	case models.Admin:
		return "Administrator"
	default:
		return "No Access" // Default fallback
	}
}

// determineMembershipStatus determines the current status of a membership based on Authentik metadata or groups
func (s *MembershipService) determineMembershipStatus(user *models.UserProfile) models.MembershipStatus {
	// Check if we have membership_status metadata from Authentik
	if user.MembershipStatus != "" {
		s.logger.Debug("Using membership_status metadata: %s for user %s", user.MembershipStatus, user.Email)
		
		// Map the status string to our enum
		switch strings.ToLower(user.MembershipStatus) {
		case "active":
			return models.StatusActive
		case "suspended":
			return models.StatusSuspended
		case "expired":
			return models.StatusExpired
		case "inactive":
			return models.StatusInactive
		}
	}
	
	// Fall back to checking groups if no metadata or unrecognized status
	// Check for specific groups that might indicate a suspended or expired status
	for _, group := range user.Groups {
		if group == "suspended-members" || group == "account-suspended" {
			return models.StatusSuspended
		}
		if group == "expired-members" || group == "account-expired" {
			return models.StatusExpired
		}
		if group == "inactive-members" {
			return models.StatusInactive
		}
	}

	// If no special status groups and they have access, they're active
	if user.AccessLevel > models.NoAccess {
		return models.StatusActive
	}

	// Default to inactive
	return models.StatusInactive
}

// getJoinDate retrieves the join date for a user
// Uses member_since metadata from Authentik if available, otherwise falls back to group-based detection
func (s *MembershipService) getJoinDate(user *models.UserProfile) *time.Time {
	// Check if we have member_since metadata from Authentik
	if user.MemberSince != "" {
		s.logger.Debug("Using member_since metadata: %s for user %s", user.MemberSince, user.Email)
		
		// Parse the date string (expected format: YYYY-MM-DD)
		memberSince, err := time.Parse("2006-01-02", user.MemberSince)
		if err == nil {
			return &memberSince
		}
		
		s.logger.Error("Failed to parse member_since date: %v", err)
	}
	
	// Default to 1 year ago if we don't have real data
	t := time.Now().AddDate(-1, 0, 0)
	return &t
}

// getExpiryDate retrieves the expiry date for a user
// Uses expiry_date metadata from Authentik if available, otherwise falls back to group-based detection
func (s *MembershipService) getExpiryDate(user *models.UserProfile) *time.Time {
	// Check if we have expiry_date metadata from Authentik
	if user.ExpiryDate != "" {
		s.logger.Debug("Using expiry_date metadata: %s for user %s", user.ExpiryDate, user.Email)
		
		// Parse the date string (expected format: YYYY-MM-DD)
		expiryDate, err := time.Parse("2006-01-02", user.ExpiryDate)
		if err == nil {
			return &expiryDate
		}
		
		s.logger.Error("Failed to parse expiry_date: %v", err)
	}
	
	// For annual members, expiry is 1 year from now
	if containsAny(user.Groups, []string{"annual-members"}) {
		expiry := time.Now().AddDate(1, 0, 0)
		return &expiry
	}
	
	// For lifetime members, expiry is 100 years from now (effectively forever)
	if containsAny(user.Groups, []string{"lifetime-members"}) {
		expiryDate := time.Now().AddDate(100, 0, 0)
		return &expiryDate
	}
	
	// For monthly members, expiry is 1 month from now
	if containsAny(user.Groups, []string{"monthly-members"}) {
		expiryDate := time.Now().AddDate(0, 1, 0)
		return &expiryDate
	}
	
	// Check for specific groups that indicate a specific expiry year
	if containsAny(user.Groups, []string{"expires-2023"}) {
		expiryDate := time.Date(2023, 12, 31, 23, 59, 59, 0, time.UTC)
		return &expiryDate
	}
	if containsAny(user.Groups, []string{"expires-2024"}) {
		expiryDate := time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)
		return &expiryDate
	}
	if containsAny(user.Groups, []string{"expires-2025"}) {
		expiryDate := time.Date(2025, 12, 31, 23, 59, 59, 0, time.UTC)
		return &expiryDate
	}

	// Default to 1 year from now if we don't have real data
	t := time.Now().AddDate(1, 0, 0)
	return &t
}

// containsAny checks if any of the target strings are in the source slice
func containsAny(source []string, targets []string) bool {
	for _, s := range source {
		for _, t := range targets {
			if s == t {
				return true
			}
		}
	}
	return false
}
