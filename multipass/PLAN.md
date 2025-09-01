# Multipass - Digital Makerspace ID System

## Project Overview

**Multipass** is a Go-based web application that serves as a digital ID card system for makerspace members. It functions as an SSO (Single Sign-On) service provider, allowing users to authenticate and display their membership credentials in a digital ID card format.

### Core Features
- **Digital ID Card**: Mobile-first ID card interface displaying member information
- **SSO Service Provider**: Integration with web SSO systems for authentication
- **Responsive Design**: Optimized layouts for both mobile and desktop
- **Membership Management**: Flexible member data and access control

## Technical Architecture

### Technology Stack
- **Backend**: Go with Gin web framework
- **Data Source**: Authentik reverse proxy headers + API for extended data
- **Authentication**: Authentik reverse proxy with header-based user data
- **Frontend**: Server-side rendered HTML with HTMX for interactivity
- **Styling**: Tailwind CSS for responsive design
- **Deployment**: Docker Compose

### Core Dependencies
```go
// go.mod
module multipass

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/go-resty/resty/v2 v2.7.0  // For Authentik API calls
    gopkg.in/yaml.v3 v3.0.1             // Configuration
    github.com/golang/freetype v0.0.0-20170609003504-e2365dfdc4a0  // QR code generation
)
```

## Project Structure

```
multipass/
├── cmd/
│   └── multipass/
│       └── main.go            # Application entry point
├── internal/
│   ├── config/
│   │   └── config.go          # Configuration management
│   ├── models/
│   │   ├── user.go            # User/member data models
│   │   └── membership.go      # Membership status and permissions
│   ├── handlers/
│   │   ├── auth.go            # SSO authentication handlers
│   │   ├── profile.go         # User profile management
│   │   └── card.go            # ID card display handlers
│   ├── services/
│   │   ├── sso.go             # SSO service provider logic
│   │   ├── membership.go      # Membership validation
│   │   └── card_generator.go  # ID card data preparation
│   └── middleware/
│       └── auth.go            # Authentication middleware
├── web/
│   ├── templates/
│   │   ├── base.html          # Base template
│   │   ├── card_mobile.html   # Mobile ID card layout
│   │   ├── card_desktop.html  # Desktop ID card layout
│   │   └── login.html         # SSO login page
│   └── static/
│       ├── css/
│       │   └── styles.css     # Custom styles
│       ├── js/
│       │   └── card.js        # ID card interactions
│       └── images/
│           └── logo.png       # Makerspace logo
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

## Data Model

### User Data from Authentik
The application retrieves user information directly from Authentik via SSO claims and API calls:

```go
type UserProfile struct {
    Email       string   `json:"email"`
    FirstName   string   `json:"first_name"`
    LastName    string   `json:"last_name"`
    Groups      []string `json:"groups"`          // Membership types from Authentik groups
    Avatar      *string  `json:"avatar,omitempty"` // Profile image URL from Authentik
    Phone       *string  `json:"phone,omitempty"`
    MemberID    string   `json:"member_id"`        // Generated from email or Authentik user ID
    AccessLevel UserLevel `json:"access_level"`    // Derived from group membership
}

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

type MembershipInfo struct {
    MembershipType      string              `json:"membership_type"`       // Derived from Authentik groups
    Status              MembershipStatus    `json:"status"`                // Active if user has valid groups
    AccessPermissions   []string            `json:"access_permissions"`    // Equipment access from group attributes
    UserLevel           UserLevel           `json:"user_level"`            // Limited Volunteer or Full Member
}

type MembershipStatus int

const (
    StatusInactive MembershipStatus = iota
    StatusActive
    StatusSuspended
    StatusExpired
)
```

### Authentication Flow
Authentik reverse proxy provides user data via headers:
- User authenticated by Authentik reverse proxy
- Basic user data extracted from HTTP headers
- Extended user data fetched from Authentik API as needed
- No session storage required - stateless per request

## UI/UX Design Requirements

### Mobile ID Card Layout
- **Portrait orientation optimized**
- **Card-like visual design** with rounded corners and shadow
- **Member photo** prominently displayed at top
- **Essential information**:
  - Full name (large, bold)
  - Membership type and status
  - Member ID number
  - QR code for quick scanning
  - Expiration date
- **Color coding** by membership type
- **Touch-friendly** interactions for card flipping/details

### Desktop ID Card Layout
- **Landscape card format** similar to physical ID cards
- **Left side**: Member photo and QR code
- **Right side**: Member details and access information
- **Additional information** visible without interaction:
  - Emergency contact
  - Access permissions
  - Recent activity
- **Hover effects** for enhanced interactivity

### Responsive Breakpoints
- **Mobile**: < 768px (card stacked vertically)
- **Tablet**: 768px - 1024px (compact card layout)
- **Desktop**: > 1024px (full horizontal card layout)

## Authentik Integration

### Reverse Proxy Headers
Authentik reverse proxy provides user data via HTTP headers:
- `X-Authentik-Email`: User email address
- `X-Authentik-Name`: Full name
- `X-Authentik-Given-Name`: First name
- `X-Authentik-Family-Name`: Last name
- `X-Authentik-Username`: Username
- `X-Authentik-Groups`: Comma-separated group list
- `X-Authentik-User-Id`: Authentik user ID

### Header Processing
```go
type UserFromHeaders struct {
    Email     string   `json:"email"`
    FirstName string   `json:"first_name"`
    LastName  string   `json:"last_name"`
    Username  string   `json:"username"`
    UserID    string   `json:"user_id"`
    Groups    []string `json:"groups"`
}

// Group mapping configuration loaded from a YAML file
type GroupMappingConfig struct {
    Mappings map[string]string `yaml:"mappings"` // Maps Authentik group names to access levels
    DefaultLevel string `yaml:"default_level"` // Default access level if no matching groups found
}

// Group mapping for membership types and user levels
// This mapping will be loaded from a configuration file
var GroupMapping = map[string]UserLevel{
    "volunteers-limited":   LimitedVolunteer,
    "members-full":        FullMember,
    "staff":              Staff,
    "admin":              Admin,
}

// DetermineUserLevel determines user level from Authentik groups
func DetermineUserLevel(groups []string) UserLevel {
    // Check for highest privilege level first
    for _, group := range groups {
        if level, exists := GroupMapping[group]; exists {
            return level
        }
    }
    return NoAccess // Default to No Access if no matching groups found
}

// Debug middleware will have no groups, resulting in "No Access" level
// This ensures proper testing of the default access level
```

### Authentik API Integration
For extended user data not available in headers:
- Profile images via `/api/v3/core/users/{user_id}/`
- Group details via `/api/v3/core/groups/`
- User attributes and custom fields

## Development Phases

### Phase 1: Core Infrastructure (Week 1-2)
- [ ] Set up Go project with Gin framework
- [ ] Header extraction middleware for Authentik reverse proxy
- [ ] User profile data extraction from headers
- [ ] Authentik API client for extended data
- [ ] Implement user level determination logic

### Phase 2: ID Card Interface (Week 3-4)
- [ ] Mobile ID card layout
- [ ] Desktop ID card layout
- [ ] Responsive design implementation
- [ ] Member data display

### Phase 3: Enhanced Integration (Week 5-6)
- [ ] Profile image fetching from Authentik API
- [ ] Group-based membership type determination
- [ ] Caching for API responses
- [ ] Error handling for missing headers/API failures

### Phase 4: Enhancement & Polish (Week 7-8)
- [ ] QR code generation for member scanning
- [ ] Profile image upload and management
- [ ] Admin interface for membership management
- [ ] Audit logging and security hardening

## Configuration

### Environment Variables
```bash
# Authentik Integration
AUTHENTIK_URL=https://login.sequoia.garden
AUTHENTIK_API_TOKEN=your-api-token  # For extended user data API calls

# Application
RUST_LOG=info
BIND_ADDRESS=0.0.0.0:3000
MAKERSPACE_NAME="Sequoia Fabrica"
MAKERSPACE_LOGO_URL="/static/images/logo.png"
TRUSTED_PROXY_HEADERS=true  # Enable header-based authentication
GROUP_MAPPING_CONFIG="./config/group_mapping.yaml"  # Path to group mapping configuration file
```

### Group Mapping Configuration
The application uses a YAML configuration file to map Authentik groups to access levels. This file is loaded at startup and can be specified via the `GROUP_MAPPING_CONFIG` environment variable.

```yaml
# Example group_mapping.yaml
mappings:
  volunteers-limited: "LimitedVolunteer"
  members-full: "FullMember"
  staff: "Staff"
  admin: "Admin"
default_level: "NoAccess"  # Default access level if no matching groups found
```

## Security Considerations

- **HTTPS only** in production
- **CSRF protection** for state-changing operations
- **Header validation** to ensure requests come from Authentik proxy
- **Input validation** and sanitization
- **Rate limiting** on API endpoints
- **Audit logging** for access events
- **Stateless architecture** with no session storage required

## Deployment

### Docker Compose Configuration
```yaml
version: '3.8'

services:
  multipass:
    build: .
    ports:
      - "3000:3000"
    environment:
      - AUTHENTIK_URL=${AUTHENTIK_URL}
      - AUTHENTIK_API_TOKEN=${AUTHENTIK_API_TOKEN}
      - RUST_LOG=info
      - MAKERSPACE_NAME=Sequoia Fabrica
      - TRUSTED_PROXY_HEADERS=true
    volumes:
      - ./ssl:/etc/ssl:ro
    restart: unless-stopped

  # Optional: Reverse proxy
  caddy:
    image: caddy:2-alpine
    ports:
      - "80:80"
      - "443:443"
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile
      - caddy_data:/data
      - caddy_config:/config
    restart: unless-stopped

volumes:
  caddy_data:
  caddy_config:
```

### Dockerfile
```dockerfile
# Build stage
FROM golang:1.21-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o multipass cmd/multipass/main.go

# Runtime stage
FROM alpine:latest
RUN apk --no-cache add ca-certificates
WORKDIR /root/
COPY --from=builder /app/multipass ./
COPY --from=builder /app/web ./web
EXPOSE 3000
CMD ["./multipass"]
```

### Caddyfile (Optional Reverse Proxy)
```
multipass.sequoia.garden {
    reverse_proxy multipass:3000
    tls your-email@example.com
}
```

## Testing Strategy

- **Unit tests** for business logic
- **Integration tests** for Authentik SSO flows
- **End-to-end tests** for user journeys
- **Visual regression tests** for ID card layouts
- **Security testing** for authentication flows
- **API integration tests** for Authentik data retrieval

## Success Metrics

- **User adoption**: Active daily users
- **Authentication success rate**: > 99%
- **Mobile usability**: Card load time < 2s
- **SSO integration**: Seamless login experience
- **Member satisfaction**: Positive feedback on digital ID experience
