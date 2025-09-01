package services

import (
	"multipass/internal/models"
	"testing"
	"time"
)

func TestMembershipService_DetermineMembershipStatus(t *testing.T) {
	// Create a membership service
	service := &MembershipService{}

	// Test cases
	testCases := []struct {
		name             string
		groups           []string
		accessLevel      models.UserLevel
		membershipStatus string
		expectedStatus   models.MembershipStatus
	}{
		{
			name:             "Active member from group",
			groups:           []string{"members"},
			accessLevel:      models.FullMember,
			membershipStatus: "",
			expectedStatus:   models.StatusActive,
		},
		{
			name:             "Suspended member from group",
			groups:           []string{"suspended-members"},
			accessLevel:      models.FullMember,
			membershipStatus: "",
			expectedStatus:   models.StatusSuspended,
		},
		{
			name:             "Expired member from group",
			groups:           []string{"expired-members"},
			accessLevel:      models.FullMember,
			membershipStatus: "",
			expectedStatus:   models.StatusExpired,
		},
		{
			name:             "Inactive member from group",
			groups:           []string{"inactive-members"},
			accessLevel:      models.FullMember,
			membershipStatus: "",
			expectedStatus:   models.StatusInactive,
		},
		{
			name:             "No access",
			groups:           []string{},
			accessLevel:      models.NoAccess,
			membershipStatus: "",
			expectedStatus:   models.StatusInactive,
		},
		{
			name:             "Active member from metadata",
			groups:           []string{},
			accessLevel:      models.NoAccess,
			membershipStatus: "active",
			expectedStatus:   models.StatusActive,
		},
		{
			name:             "Suspended member from metadata",
			groups:           []string{},
			accessLevel:      models.FullMember,
			membershipStatus: "suspended",
			expectedStatus:   models.StatusSuspended,
		},
		{
			name:             "Expired member from metadata",
			groups:           []string{},
			accessLevel:      models.FullMember,
			membershipStatus: "expired",
			expectedStatus:   models.StatusExpired,
		},
		{
			name:             "Inactive member from metadata",
			groups:           []string{},
			accessLevel:      models.FullMember,
			membershipStatus: "inactive",
			expectedStatus:   models.StatusInactive,
		},
		{
			name:             "Metadata overrides group",
			groups:           []string{"expired-members"},
			accessLevel:      models.FullMember,
			membershipStatus: "active",
			expectedStatus:   models.StatusActive,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user := &models.UserProfile{
				Groups:           tc.groups,
				AccessLevel:      tc.accessLevel,
				MembershipStatus: tc.membershipStatus,
			}
			status := service.determineMembershipStatus(user)
			if status != tc.expectedStatus {
				t.Errorf("Expected status %v, got %v", tc.expectedStatus, status)
			}
		})
	}
}

func TestMembershipService_GetJoinDate(t *testing.T) {
	// Create a membership service
	service := &MembershipService{}

	// Test cases
	testCases := []struct {
		name          string
		groups        []string
		memberSince   string
		expectedYear  int
		expectedMonth time.Month
		expectedDay   int
	}{
		{
			name:          "Default join date (no metadata)",
			groups:        []string{"members"},
			memberSince:   "",
			expectedYear:  time.Now().Year() - 1, // Default is 1 year ago
			expectedMonth: time.Now().Month(),
			expectedDay:   time.Now().Day(),
		},
		{
			name:          "Join date from metadata",
			groups:        []string{"members"},
			memberSince:   "2023-05-15",
			expectedYear:  2023,
			expectedMonth: time.May,
			expectedDay:   15,
		},
		{
			name:          "Invalid metadata format",
			groups:        []string{"members"},
			memberSince:   "invalid-date",
			expectedYear:  time.Now().Year() - 1, // Default is 1 year ago
			expectedMonth: time.Now().Month(),
			expectedDay:   time.Now().Day(),
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user := &models.UserProfile{
				Groups:      tc.groups,
				MemberSince: tc.memberSince,
			}
			joinDate := service.getJoinDate(user)

			// For the default case (1 year ago), we just check the year
			if tc.name == "No join date in groups" {
				if joinDate.Year() != tc.expectedYear {
					t.Errorf("Expected year %d, got %d", tc.expectedYear, joinDate.Year())
				}
			} else {
				// For specific dates, check year, month, day
				if joinDate.Year() != tc.expectedYear ||
				   joinDate.Month() != tc.expectedMonth ||
				   joinDate.Day() != tc.expectedDay {
					t.Errorf("Expected date %d-%d-%d, got %d-%d-%d",
						tc.expectedYear, tc.expectedMonth, tc.expectedDay,
						joinDate.Year(), joinDate.Month(), joinDate.Day())
				}
			}
		})
	}
}

func TestMembershipService_GetExpiryDate(t *testing.T) {
	// Create a membership service
	service := &MembershipService{}

	// Test cases
	testCases := []struct {
		name          string
		groups        []string
		expiryDate    string
		expectedYear  int
		expectedMonth time.Month
		expectedDay   int
	}{
		{
			name:          "Annual member from group",
			groups:        []string{"annual-members"},
			expiryDate:    "",
			expectedYear:  time.Now().Year() + 1, // 1 year from now (approximate)
			expectedMonth: time.Now().Month(),
			expectedDay:   time.Now().Day(),
		},
		{
			name:          "Monthly member from group",
			groups:        []string{"monthly-members"},
			expiryDate:    "",
			expectedYear:  time.Now().Year(),
			expectedMonth: time.Now().Month() + 1, // 1 month from now
			expectedDay:   time.Now().Day(),
		},
		{
			name:          "Expires in 2024 from group",
			groups:        []string{"expires-2024"},
			expiryDate:    "",
			expectedYear:  2024,
			expectedMonth: time.December,
			expectedDay:   31,
		},
		{
			name:          "Expiry date from metadata",
			groups:        []string{"members"},
			expiryDate:    "2025-06-30",
			expectedYear:  2025,
			expectedMonth: time.June,
			expectedDay:   30,
		},
		{
			name:          "Metadata overrides group",
			groups:        []string{"expires-2024"},
			expiryDate:    "2025-06-30",
			expectedYear:  2025,
			expectedMonth: time.June,
			expectedDay:   30,
		},
		{
			name:          "Invalid metadata format",
			groups:        []string{"expires-2024"},
			expiryDate:    "invalid-date",
			expectedYear:  2024, // Falls back to group
			expectedMonth: time.December,
			expectedDay:   31,
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user := &models.UserProfile{
				Groups:     tc.groups,
				ExpiryDate: tc.expiryDate,
			}
			expiryDate := service.getExpiryDate(user)

			// For monthly members, we just check that it's roughly a month from now
			if tc.name == "Monthly member from group" {
				expectedDate := time.Now().AddDate(0, 1, 0)
				dayDiff := expiryDate.Sub(expectedDate).Hours() / 24
				if dayDiff < -1 || dayDiff > 1 { // Allow 1 day difference due to test execution timing
					t.Errorf("Expected date around %v, got %v", expectedDate, expiryDate)
				}
			} else {
				// For specific dates, check year, month, day
				if expiryDate.Year() != tc.expectedYear ||
				   expiryDate.Month() != tc.expectedMonth ||
				   expiryDate.Day() != tc.expectedDay {
					t.Errorf("Expected date %d-%d-%d, got %d-%d-%d",
						tc.expectedYear, tc.expectedMonth, tc.expectedDay,
						expiryDate.Year(), expiryDate.Month(), expiryDate.Day())
				}
			}
		})
	}
}
