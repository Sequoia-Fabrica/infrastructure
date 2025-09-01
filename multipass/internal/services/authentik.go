package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"multipass/internal/config"
	"multipass/internal/models"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
)

// AuthentikClient provides methods to interact with the Authentik API
type AuthentikClient struct {
	client    *resty.Client
	baseURL   string
	apiToken  string
	userCache map[string]*models.UserProfile
	cacheTTL  time.Duration
}

// AuthentikUserResponse represents the response from Authentik's user API
type AuthentikUserResponse struct {
	ID         int                    `json:"pk"`
	Username   string                 `json:"username"`
	Name       string                 `json:"name"`
	Email      string                 `json:"email"`
	IsActive   bool                   `json:"is_active"`
	LastLogin  string                 `json:"last_login"`
	Groups     []string               `json:"groups"`
	Avatar     string                 `json:"avatar"`
	Attributes map[string]interface{} `json:"attributes"`
}

// NewAuthentikClient creates a new Authentik API client
func NewAuthentikClient(cfg *config.Config) *AuthentikClient {
	// Create HTTP client with timeout
	client := resty.New().SetTimeout(10 * time.Second)

	// Set headers
	client.SetHeader("Accept", "application/json")

	// Enable debug mode
	if cfg.DebugMode {
		log.Printf("[DEBUG] Authentik client initialized with debug mode enabled")
		log.Printf("[DEBUG] Authentik URL: %s", cfg.AuthentikURL)

		client.SetDebug(true)
		client.OnBeforeRequest(func(c *resty.Client, r *resty.Request) error {
			log.Printf("[DEBUG] Authentik API Request: %s %s", r.Method, r.URL)
			log.Printf("[DEBUG] Request Headers: %v", r.Header)
			return nil
		})
		client.OnAfterResponse(func(c *resty.Client, r *resty.Response) error {
			log.Printf("[DEBUG] Authentik API Response: Status=%d, Time=%v",
				r.StatusCode(), r.Time())

			// Only log response body if it's JSON or plain text
			contentType := r.Header().Get("Content-Type")
			if (contentType == "application/json" || contentType == "text/plain") && r.Body() != nil {
				if len(r.Body()) < 500 { // Only log if body is reasonably small
					log.Printf("[DEBUG] Response Body: %s", string(r.Body()))
				} else {
					log.Printf("[DEBUG] Response Body: (large %d bytes, first 500) %s...",
						len(r.Body()), string(r.Body()[:500]))
				}
			} else {
				log.Printf("[DEBUG] Response Content-Type: %s, Size: %d bytes",
					contentType, len(r.Body()))
			}
			return nil
		})
	}

	// Set API token if available
	if cfg.AuthentikAPIToken != "" {
		client.SetAuthToken(cfg.AuthentikAPIToken)
		log.Printf("[DEBUG] Authentik API Token configured (length: %d)", len(cfg.AuthentikAPIToken))
	} else {
		log.Printf("[DEBUG] Warning: No Authentik API Token configured")
	}

	return &AuthentikClient{
		baseURL:   cfg.AuthentikURL,
		client:    client,
		userCache: make(map[string]*models.UserProfile),
		cacheTTL:  15 * time.Minute,
	}
}

// GetUserByID retrieves user information from Authentik by user ID
func (ac *AuthentikClient) GetUserByID(userID string) (*models.UserProfile, error) {
	log.Printf("[DEBUG] GetUserByID called with userID: %s", userID)

	// Check cache first
	if cachedUser, ok := ac.userCache[userID]; ok {
		log.Printf("[DEBUG] User found in cache: %s (%s)", cachedUser.FullName, cachedUser.Email)
		return cachedUser, nil
	}

	// Try to convert string ID to integer if it's numeric
	var numericID int
	_, err := fmt.Sscanf(userID, "%d", &numericID)
	if err != nil {
		// If not numeric, it might be an email or username
		log.Printf("[DEBUG] UserID is not numeric, trying as username/email: %s", userID)
	}

	// Make API request to Authentik
	url := fmt.Sprintf("%s/api/v3/core/users/%s/", ac.baseURL, userID)
	log.Printf("[DEBUG] Making Authentik API request to: %s", url)

	resp, err := ac.client.R().Get(url)
	if err != nil {
		log.Printf("[ERROR] Failed to request user data: %v", err)
		return nil, fmt.Errorf("failed to request user data: %w", err)
	}

	log.Printf("[DEBUG] Authentik API response status: %d", resp.StatusCode())

	if resp.StatusCode() != http.StatusOK {
		// Only log a portion of the response body for non-JSON responses
		contentType := resp.Header().Get("Content-Type")
		var bodyExcerpt string
		if contentType == "application/json" || contentType == "text/plain" {
			if len(resp.Body()) > 500 {
				bodyExcerpt = string(resp.Body()[:500]) + "..."
			} else {
				bodyExcerpt = string(resp.Body())
			}
		} else {
			bodyExcerpt = fmt.Sprintf("[%s content, %d bytes]", contentType, len(resp.Body()))
		}

		log.Printf("[ERROR] Failed to get user data, status: %d, body: %s",
			resp.StatusCode(), bodyExcerpt)
		return nil, fmt.Errorf("failed to get user data, status: %d", resp.StatusCode())
	}

	// Parse response
	var authUser AuthentikUserResponse
	if err := json.Unmarshal(resp.Body(), &authUser); err != nil {
		log.Printf("[ERROR] Failed to parse user data: %v", err)
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	log.Printf("[DEBUG] Successfully parsed user data for: %s (%s)",
		authUser.Name, authUser.Email)

	// Use the helper function to create user profile
	userProfile, err := createUserProfileFromAuthentikUser(ac, authUser)
	if err != nil {
		log.Printf("[ERROR] Error creating user profile: %v", err)
		return nil, err
	}

	// Cache the user
	ac.userCache[userID] = userProfile

	return userProfile, nil
}

// GetUserByEmail retrieves user information from Authentik by email
func (ac *AuthentikClient) GetUserByEmail(email string) (*models.UserProfile, error) {
	log.Printf("[DEBUG] GetUserByEmail called with email: %s", email)

	// Make API request to Authentik
	url := fmt.Sprintf("%s/api/v3/core/users/", ac.baseURL)
	log.Printf("[DEBUG] Making Authentik API request to: %s with email filter", url)

	resp, err := ac.client.R().SetQueryParam("email", email).Get(url)
	if err != nil {
		log.Printf("[ERROR] Failed to request user data by email: %v", err)
		return nil, fmt.Errorf("failed to request user data: %w", err)
	}

	log.Printf("[DEBUG] Authentik API response status for email search: %d", resp.StatusCode())

	if resp.StatusCode() != http.StatusOK {
		// Only log a portion of the response body for non-JSON responses
		contentType := resp.Header().Get("Content-Type")
		var bodyExcerpt string
		if contentType == "application/json" || contentType == "text/plain" {
			if len(resp.Body()) > 500 {
				bodyExcerpt = string(resp.Body()[:500]) + "..."
			} else {
				bodyExcerpt = string(resp.Body())
			}
		} else {
			bodyExcerpt = fmt.Sprintf("[%s content, %d bytes]", contentType, len(resp.Body()))
		}

		log.Printf("[ERROR] Failed to get user data by email, status: %d, body: %s",
			resp.StatusCode(), bodyExcerpt)
		return nil, fmt.Errorf("failed to get user data, status: %d", resp.StatusCode())
	}

	// Log the response body for debugging
	log.Printf("[DEBUG] Response body: %s", string(resp.Body())[:min(len(resp.Body()), 200)])

	// First try to parse as a paginated response
	var paginatedResponse struct {
		Count    int                     `json:"count"`
		Next     *string                 `json:"next"`
		Previous *string                 `json:"previous"`
		Results  []AuthentikUserResponse `json:"results"`
	}

	if err := json.Unmarshal(resp.Body(), &paginatedResponse); err != nil {
		log.Printf("[ERROR] Failed to parse paginated response: %v", err)

		// Try parsing as a direct array
		var authUsers []AuthentikUserResponse
		if err := json.Unmarshal(resp.Body(), &authUsers); err != nil {
			log.Printf("[ERROR] Failed to parse user data as array: %v", err)
			return nil, fmt.Errorf("failed to parse user data: %w", err)
		}

		// Check if user was found
		if len(authUsers) == 0 {
			log.Printf("[ERROR] No user found with email: %s", email)
			return nil, errors.New("user not found")
		}

		// Use the first user from the array
		authUser := authUsers[0]
		log.Printf("[DEBUG] Found user by email in array response: %s (ID: %d)", authUser.Name, authUser.ID)
		return createUserProfileFromAuthentikUser(ac, authUser)
	}

	// Check if we have any results
	if len(paginatedResponse.Results) == 0 {
		log.Printf("[ERROR] No user found with email: %s in paginated response", email)
		return nil, errors.New("user not found")
	}

	// Get first user from paginated response
	authUser := paginatedResponse.Results[0]
	log.Printf("[DEBUG] Found user by email in paginated response: %s (ID: %d)", authUser.Name, authUser.ID)
	return createUserProfileFromAuthentikUser(ac, authUser)
}

// Helper function to create a user profile from an Authentik user
func createUserProfileFromAuthentikUser(ac *AuthentikClient, authUser AuthentikUserResponse) (*models.UserProfile, error) {
	// Get user groups
	log.Printf("[DEBUG] Fetching user groups for user ID: %d", authUser.ID)
	groups, err := ac.GetUserGroups(authUser.ID)
	if err != nil {
		// Log error but continue
		log.Printf("[ERROR] Error getting user groups: %v", err)
	}

	// Convert the integer ID to string for the member ID
	authentikUID := fmt.Sprintf("%d", authUser.ID)

	// Create user profile with the numeric PK directly as the Member ID
	userProfile := &models.UserProfile{
		Email:       authUser.Email,
		FullName:    authUser.Name,
		Groups:      groups,
		MemberID:    fmt.Sprintf("%d", authUser.ID), // Use the numeric PK directly
		AccessLevel: models.DetermineUserLevel(groups),
		AuthentikID: authentikUID,
	}

	// Add avatar if available
	if authUser.Avatar != "" {
		userProfile.Avatar = &authUser.Avatar
	}

	// Extract user attributes/metadata
	if authUser.Attributes != nil {
		// Log available attributes for debugging
		log.Printf("[DEBUG] User attributes for %s: %v", authUser.Email, authUser.Attributes)

		// Extract member_since if available
		if memberSince, ok := authUser.Attributes["member_since"].(string); ok {
			log.Printf("[DEBUG] Found member_since attribute: %s", memberSince)
			userProfile.MemberSince = memberSince
		}
		
		// Extract membership_type if available
		if membershipType, ok := authUser.Attributes["membership_type"].(string); ok {
			log.Printf("[DEBUG] Found membership_type attribute: %s", membershipType)
			userProfile.MembershipType = membershipType
		}
		
		// Extract expiry_date if available
		if expiryDate, ok := authUser.Attributes["expiry_date"].(string); ok {
			log.Printf("[DEBUG] Found expiry_date attribute: %s", expiryDate)
			userProfile.ExpiryDate = expiryDate
		}
		
		// Extract membership_status if available
		if status, ok := authUser.Attributes["membership_status"].(string); ok {
			log.Printf("[DEBUG] Found membership_status attribute: %s", status)
			userProfile.MembershipStatus = status
		}
	}

	return userProfile, nil
}

// Helper function to get min of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// GetUserGroups retrieves user groups from Authentik
func (ac *AuthentikClient) GetUserGroups(userID int) ([]string, error) {
	log.Printf("[DEBUG] GetUserGroups called with userID: %d", userID)

	// Make API request to Authentik
	// Try using the groups endpoint with a filter for the user
	url := fmt.Sprintf("%s/api/v3/core/groups/?user=%d", ac.baseURL, userID)
	log.Printf("[DEBUG] Making Authentik API request for groups to: %s", url)

	resp, err := ac.client.R().Get(url)
	if err != nil {
		log.Printf("[ERROR] Failed to request user groups: %v", err)
		return nil, fmt.Errorf("failed to request user data: %w", err)
	}

	log.Printf("[DEBUG] Authentik API response status for groups: %d", resp.StatusCode())

	if resp.StatusCode() != http.StatusOK {
		// Only log a portion of the response body for non-JSON responses
		contentType := resp.Header().Get("Content-Type")
		var bodyExcerpt string
		if contentType == "application/json" || contentType == "text/plain" {
			if len(resp.Body()) > 500 {
				bodyExcerpt = string(resp.Body()[:500]) + "..."
			} else {
				bodyExcerpt = string(resp.Body())
			}
		} else {
			bodyExcerpt = fmt.Sprintf("[%s content, %d bytes]", contentType, len(resp.Body()))
		}

		log.Printf("[ERROR] Failed to get user groups, status: %d, body: %s",
			resp.StatusCode(), bodyExcerpt)
		return nil, fmt.Errorf("failed to get user data, status: %d", resp.StatusCode())
	}

	// Parse response
	var response struct {
		Results []struct {
			Name string `json:"name"`
		} `json:"results"`
	}

	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		log.Printf("[ERROR] Failed to parse user groups data: %v", err)
		return nil, fmt.Errorf("failed to parse user data: %w", err)
	}

	log.Printf("[DEBUG] Retrieved %d groups for user ID %d", len(response.Results), userID)
	if len(response.Results) > 0 {
		log.Printf("[DEBUG] Groups: %v", response.Results)
	}

	// Extract group names
	groups := make([]string, 0, len(response.Results))
	for _, group := range response.Results {
		groups = append(groups, group.Name)
	}

	return groups, nil
}

// No longer needed as we're using the numeric PK directly
