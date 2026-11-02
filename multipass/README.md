# Multipass - Digital Makerspace ID System

A Go-based web application that serves as a digital ID card system for makerspace members, integrating with Authentik SSO for authentication and providing mobile-first digital membership cards.

## Release
A container image is built in GitHub Actions. Subsequently, the ansible YAML is updated.

## Features

- **Digital ID Cards**: Responsive membership cards that adapt to any device
- **Authentik SSO Integration**: Seamless authentication via reverse proxy headers
- **Two-Tier User System**: Limited Volunteer and Full Member access levels
- **Responsive Design**: Optimized for both mobile and desktop viewing
- **QR Code Support**: Digital verification codes for member scanning
- **Real-time Permissions**: Dynamic access control based on group membership

## User Levels

- **Limited Volunteer**: Basic workspace access with supervised equipment use
- **Full Member**: Complete workspace access with independent equipment operation

## Quick Start

### Using Docker Compose (Recommended)

1. **Clone and setup**:
   ```bash
   git clone <repository-url>
   cd multipass
   cp .env.example .env
   ```

2. **Configure environment**:
   Edit `.env` file with your Authentik settings:
   ```bash
   AUTHENTIK_URL=https://your-authentik-instance.com
   AUTHENTIK_API_TOKEN=your-api-token
   MAKERSPACE_NAME="Your Makerspace Name"
   ```

3. **Start the application**:
   ```bash
   docker-compose up -d
   ```

4. **Access the application**:
   - Application: http://localhost:3000
   - Health check: http://localhost:3000/health

### With Reverse Proxy (Production)

For production deployment with SSL termination:

```bash
docker-compose --profile reverse-proxy up -d
```

This starts both the application and Caddy reverse proxy with automatic HTTPS.

### Development Setup

1. **Install Go 1.21+ and Node.js 16+**
2. **Clone repository and install dependencies**:
   ```bash
   git clone <repository-url>
   cd multipass
   go mod download
   npm install
   ```

3. **Set environment variables**:
   ```bash
   export ENVIRONMENT=development
   export AUTHENTIK_URL=https://your-authentik-instance.com
   export MAKERSPACE_NAME="Your Makerspace"
   ```

4. **Build CSS with Tailwind**:
   ```bash
   # Build CSS once
   make css

   # Or watch for CSS changes during development
   make css-watch
   ```

5. **Run the application**:
   ```bash
   # Run with pre-built CSS
   go run cmd/multipass/main.go

   # Or use the dev command to build CSS and run
   make dev
   ```

## Configuration

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `PORT` | `3000` | Server port |
| `BIND_ADDRESS` | `0.0.0.0` | Server bind address |
| `ENVIRONMENT` | `development` | Environment mode (development/production) |
| `AUTHENTIK_URL` | `https://login.sequoia.garden` | Authentik instance URL |
| `AUTHENTIK_API_TOKEN` | - | Authentik API token for extended data |
| `TRUSTED_PROXY_HEADERS` | `true` | Enable header-based authentication |
| `GROUP_MAPPING_CONFIG` | `./config/group_mapping.yaml` | Path to group mapping configuration file |
| `MAKERSPACE_NAME` | `Sequoia Fabrica` | Your makerspace name |
| `MAKERSPACE_LOGO_URL` | `/static/images/logo.png` | Logo URL |
| `DEBUG_MODE` | `false` | Enable debug mode |
| `TOKEN_SECRET` | - | Secret key for generating and validating secure tokens for public card access |
| `CSRF_ENABLED` | `true` | Enable CSRF protection |
| `RATE_LIMIT` | `100` | Rate limit per minute |

### Authentik Integration

Multipass integrates with Authentik for authentication and user management in two ways:

### 1. Reverse Proxy Headers

For standard authenticated access, Multipass expects the following headers from the Authentik reverse proxy:

- `X-Authentik-Email`: User's email address
- `X-Authentik-Name`: User's full name
- `X-Authentik-Groups`: User's groups (pipe-separated)

These headers are used to create a user profile and determine access levels based on group membership.

### 2. API Integration

For token-based public access, Multipass uses the Authentik API to retrieve user information. This requires:

- `AUTHENTIK_URL`: The URL of your Authentik instance
- `AUTHENTIK_API_TOKEN`: An API token with permissions to read user data

## Token-Based Authentication

Multipass supports secure token-based authentication for public access to digital ID cards. This allows members to share their digital ID card via QR code or URL without requiring the recipient to log in.

### How It Works

1. **Token Generation**: Authenticated users can generate a secure token by visiting `/generate-token` or `/share`
2. **Token Security**: Tokens are secured using HMAC-SHA256 with a server-side secret key
3. **Token Format**: `base64(userID:email:timestamp):hmac_signature`
4. **Token Validation**: When a token is presented, Multipass validates the signature and expiration
5. **User Lookup**: After validation, Multipass uses the Authentik API to retrieve the user's information
6. **Card Display**: The user's digital ID card is displayed without requiring authentication

### Configuration

To enable token-based authentication:

1. Set the `TOKEN_SECRET` environment variable to a secure random string
2. Configure the `AUTHENTIK_URL` and `AUTHENTIK_API_TOKEN` environment variables
3. Ensure your Authentik API token has permissions to read user data

### Security Considerations

- Tokens expire after 30 days by default
- Each token is tied to a specific user and cannot be used for other users
- The token signature is verified using HMAC to prevent tampering
- The server-side secret key should be kept secure and rotated periodically

## Wooden Badges

Members make their own wooden ID badges at `/badge`: one click issues a badge and gives them laser files for the xTool P2. Staff can work on anyone's with `/badge?member=<id>`.

- **The front**: a portrait CR80 (54 × 85.6 mm) SVG in millimetres, with a guilloche border, a rosette whose pattern is seeded by the badge's code, microtext (a straight line and a ring around the rosette carrying the member's name, number, badge serial and code), and a QR code.
- **The back** (optional): a return address and a signed PDF417 that ID scanners read. Engrave it after cutting the front by flipping the card left to right in the hole it left; the grey outline is there to line it up.
- **Layers by color**, for xTool Creative Space: blue guilloche and green lettering are scored, the black QR and PDF417 are fill-engraved, the red outline and lanyard slot are cut, and the grey outline is not output. Lettering uses a built-in single-stroke font, which stays legible on wood far smaller than filled outlines.

### Badge lifecycle

Every badge has a serial, recorded in the badge store (`BADGE_DB`, a JSON file). Making a badge records it as **lasered**, and its QR code works at once. A member holds one active badge at a time. From `/badge` they can mark it:

- **failed**, when the laser job went wrong, or
- **lost**, which stops it working for good. Scanning a lost badge asks the finder to return it, and it appears in the published revocation list, so offline verifiers learn of it too.

Either way they can then make a new one. Voiding can't be undone; a badge found again is replaced, not revived.

### What the codes say

- **QR code** (front): `HTTPS://<host>/B/<serial>/<code>`, where the code is 13 characters of an HMAC over the serial and member. It never expires; it opens the member's card with their live status from Authentik (email hidden) for as long as the badge is active. It's uppercase so the QR can use alphanumeric mode and fewer, larger modules.
- **PDF417** (back): an AAMVA DL/ID barcode, the format licence scanners and libraries read, signed as a [W3C Verifiable Credential Barcode](https://w3c.github.io/vc-barcodes/), the scheme California's DMV signs its licences with. Here the signer is the makerspace, `did:web:<host>`, with its key at `/.well-known/did.json`, derived from `BADGE_SECRET`.
  - **What scanners read**: an `ID` subfile with the member's name, member number (`DAQ`), issue date, and a document number (`DCF`) of `B<serial>-<code>` that traces a found card to its record. The issuer number (IIN) is the makerspace's own: a `9` and five digits from its DID, where every AAMVA jurisdiction's begins with `6`.
  - **What they don't**: no date of birth, expiry, height or address (all `unavl`), jurisdiction `XX`, no driving privileges. An age check fails rather than pass on a badge; it is not a government ID and doesn't claim to be one.
  - **The signature** (`ecdsa-xi-2023`, in the `ZZ` subfile's `ZZA` field) covers all 22 mandatory fields. A verifier that checks it sees the makerspace's DID as issuer, so it can't be mistaken for a state's.
  - **Status**: each badge holds a random index in the revocation lists at `/badge-status/revocation/<n>`, signed `BitstringStatusListCredential`s. Badges marked lost or failed are set there.
  - The CBOR-LD encoding and signing in `internal/vcb` reproduce the spec's test vectors byte for byte. `internal/pdf417` is a copy of boombuler/barcode's encoder that picks the symbol's shape, so the barcode, turned to run down the card, gets modules of about 0.25 mm.

Configuration:

| Variable | Default | Description |
|----------|---------|-------------|
| `BADGE_SECRET` | - | Signs badge codes and derives the barcode signing key. Required for badges. Separate from `TOKEN_SECRET` so rotating that doesn't void every badge; changing this one does. |
| `BADGE_DB` | `./data/badges.json` | The record of issued badges. Needs a writable, persistent volume. |
| `PUBLIC_URL` | `https://multipass.sequoia.garden` | Scheme and host engraved into badge codes; also the issuer's `did:web` |

The Authentik proxy must let `/B/…`, `/b/…`, `/.well-known/did.json` and `/badge-status/revocation/…` through without login; see `skip_path_regex` in `terraform/authentik/providers.tf`.

### Group Mapping

Configure these groups in Authentik to control user access levels:

- `volunteers-limited` → Limited Volunteer access
- `members-full` → Full Member access

## API Endpoints

### Public Endpoints
- `GET /health`: Health check endpoint
- `GET /login`: Login page
- `GET /public/card?token=<token>`: Public digital ID card access with secure token

### Protected Endpoints (Require Authentication)
- `GET /` - Redirects to card
- `GET /card` - Digital ID card (responsive)
- `GET /card/mobile` - Mobile ID card layout (redirects to /card for backward compatibility)
- `GET /card/desktop` - Desktop ID card layout (redirects to /card for backward compatibility)
- `GET /profile` - User profile information
- `GET /generate-token`: Generate a secure token for public card access (authenticated)
- `GET /share`: Generate a shareable link with QR code for public card access (authenticated)
- `GET /api/v1/user`: User profile API (authenticated)

### API Endpoints
- `GET /api/v1/user` - User profile data (JSON)
- `GET /api/v1/health` - Authenticated health check

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
│   │   ├── user.go            # User data models
│   │   └── membership.go      # Membership models
│   ├── handlers/
│   │   ├── auth.go            # Authentication handlers
│   │   └── card.go            # ID card handlers
│   └── middleware/
│       └── auth.go            # Authentication middleware
├── web/
│   ├── templates/
│   │   ├── base.html          # Base template
│   │   ├── card.html          # Responsive ID card (primary template)
│   │   └── login.html         # Login page
│   └── static/
│       ├── css/
│       │   └── styles.css     # Custom styles
│       ├── js/
│       │   └── card.js        # Card interactions
│       └── images/
│           └── logo.png       # Makerspace logo
├── Dockerfile
├── docker-compose.yml
├── Caddyfile
├── go.mod
└── README.md
```

## Development

### Building

```bash
# Build binary
go build -o multipass cmd/multipass/main.go

# Build Docker image
docker build -t multipass .
```

### Testing

```bash
# Run tests
go test ./...

# Test with coverage
go test -cover ./...
```

### Adding Templates

Templates use Go's `html/template` package. Place new templates in `web/templates/` and they'll be automatically loaded.

### Static Assets

Static files in `web/static/` are served at `/static/` path. Update paths in templates accordingly.

## Deployment

### Production Checklist

- [ ] Set `ENVIRONMENT=production`
- [ ] Configure proper `AUTHENTIK_URL` and `AUTHENTIK_API_TOKEN`
- [ ] Set up reverse proxy with SSL termination
- [ ] Configure proper domain in `DOMAIN` environment variable
- [ ] Enable security headers (included in Caddyfile)
- [ ] Set up monitoring and logging
- [ ] Configure backups if needed

### Docker Deployment

The application is designed to run in containers:

```bash
# Basic deployment
docker-compose up -d

# With reverse proxy and SSL
docker-compose --profile reverse-proxy up -d
```

### Kubernetes Deployment

Example deployment files can be generated from the Docker compose configuration or created manually following Kubernetes best practices.

## Security Considerations

- **Headers Only**: Authentication relies entirely on reverse proxy headers
- **HTTPS Required**: Always use HTTPS in production
- **CSRF Protection**: Enabled by default
- **Rate Limiting**: Built-in rate limiting
- **Security Headers**: Included in Caddyfile configuration
- **Non-root User**: Docker container runs as non-root user

## Troubleshooting

### Common Issues

1. **No authentication headers**: Ensure Authentik reverse proxy is properly configured
2. **Template not found**: Check template files are in `web/templates/` directory
3. **Static files not loading**: Verify `web/static/` directory structure
4. **Permission denied**: Check Docker container user permissions

### Debug Mode

Enable debug logging:

```bash
export ENVIRONMENT=development
export GIN_MODE=debug
```

### Health Checks

Monitor application health:

```bash
curl http://localhost:3000/health
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests if applicable
5. Submit a pull request

## License

[Add your license information here]

## Support

For support and questions:
- Create an issue in the repository
- Check the troubleshooting section above
- Review the configuration documentation
