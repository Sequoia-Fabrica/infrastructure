package services

import (
	"multipass/internal/models"
	"testing"
)

func TestMembershipService_GetMembershipType(t *testing.T) {
	// Create a membership service
	service := &MembershipService{}

	// Test cases
	testCases := []struct {
		name           string
		accessLevel    models.UserLevel
		membershipType string
		expectedType   string
	}{
		{
			name:           "No access from access level",
			accessLevel:    models.NoAccess,
			membershipType: "",
			expectedType:   "No Access",
		},
		{
			name:           "Limited volunteer from access level",
			accessLevel:    models.LimitedVolunteer,
			membershipType: "",
			expectedType:   "Limited Volunteer",
		},
		{
			name:           "Full member from access level",
			accessLevel:    models.FullMember,
			membershipType: "",
			expectedType:   "Full Member",
		},
		{
			name:           "Staff from access level",
			accessLevel:    models.Staff,
			membershipType: "",
			expectedType:   "Staff Member",
		},
		{
			name:           "Admin from access level",
			accessLevel:    models.Admin,
			membershipType: "",
			expectedType:   "Administrator",
		},
		{
			name:           "Membership type from metadata",
			accessLevel:    models.NoAccess,
			membershipType: "Premium Member",
			expectedType:   "Premium Member",
		},
		{
			name:           "Metadata overrides access level",
			accessLevel:    models.FullMember,
			membershipType: "Premium Member",
			expectedType:   "Premium Member",
		},
	}

	// Run tests
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			user := &models.UserProfile{
				AccessLevel:    tc.accessLevel,
				MembershipType: tc.membershipType,
			}
			membershipType := service.getMembershipType(user)
			if membershipType != tc.expectedType {
				t.Errorf("Expected membership type %s, got %s", tc.expectedType, membershipType)
			}
		})
	}
}
