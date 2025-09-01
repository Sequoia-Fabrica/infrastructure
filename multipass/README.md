# Multipass - Digital Makerspace ID System

A Go-based web application that serves as a digital ID card system for makerspace members, integrating with Authentik SSO for authentication and providing mobile-first digital membership cards.

## Features

- **Digital ID Cards**: Mobile-optimized and desktop-friendly membership cards
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
| `MAKERSPACE_NAME` | `Sequoia Fabrica` | Your makerspace name |
| `MAKERSPACE_LOGO_URL` | `/static/images/logo.png` | Logo URL |
| `CSRF_ENABLED` | `true` | Enable CSRF protection |
| `RATE_LIMIT` | `100` | Rate limit per minute |

### Authentik Integration

Multipass expects the following headers from Authentik reverse proxy:

- `X-Authentik-Email`: User email address
- `X-Authentik-Name`: Full name
- `X-Authentik-Username`: Username
- `X-Authentik-Groups`: Comma-separated group list
- `X-Authentik-User-Id`: Authentik user ID

### Group Mapping

Configure these groups in Authentik to control user access levels:

- `volunteers-limited` → Limited Volunteer access
- `members-full` → Full Member access

## API Endpoints

### Public Endpoints
- `GET /login` - Login page
- `GET /health` - Health check

### Protected Endpoints (Require Authentication)
- `GET /` - Redirects to card
- `GET /card` - Digital ID card (responsive)
- `GET /card/mobile` - Mobile ID card layout
- `GET /card/desktop` - Desktop ID card layout
- `GET /profile` - User profile information

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
│   │   ├── card_mobile.html   # Mobile ID card
│   │   ├── card_desktop.html  # Desktop ID card
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
