# Node.js build stage for CSS
FROM node:18-alpine AS css-builder

WORKDIR /app

# Copy package.json and package-lock.json
COPY package*.json ./

# Install dependencies
RUN npm install

# Copy tailwind config and CSS source
COPY tailwind.config.js ./
COPY web/static/css/input.css ./web/static/css/input.css

# Build CSS
RUN npm run build:css

# Go build stage
FROM golang:1.23-alpine AS builder

# Install git for go mod download
RUN apk add --no-cache git

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
COPY . .

# Copy the built CSS from the css-builder stage
COPY --from=css-builder /app/web/static/css/tailwind.css ./web/static/css/tailwind.css

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o multipass cmd/multipass/main.go

# Runtime stage
FROM alpine:latest

# Install ca-certificates for HTTPS requests
RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Copy the binary from builder stage
COPY --from=builder /app/multipass ./

# Copy web assets
COPY --from=builder /app/web ./web

# Create non-root user for security
RUN addgroup -g 1001 -S multipass && \
    adduser -u 1001 -S multipass -G multipass

# Change ownership of files
RUN chown -R multipass:multipass /root/

# Switch to non-root user
USER multipass

# Expose port
EXPOSE 3000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:3000/health || exit 1

# Set environment variables
ENV GIN_MODE=release
ENV PORT=3000

# Run the application
CMD ["./multipass"]
