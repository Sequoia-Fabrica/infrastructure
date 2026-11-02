.PHONY: build run dev dev-server css css-watch install

# Build the Go application
build:
	go build -o bin/multipass cmd/multipass/main.go

# Run the application
run: build
	./bin/multipass

# Run in development mode, signed in as test.user@example.com, against a
# fake Authentik (cmd/fake-authentik) and a local badge store in ./data
dev: css dev-server

dev-server:
	$(info Running in development mode on http://localhost:3000 ...)
	go build -o bin/fake-authentik ./cmd/fake-authentik
	bin/fake-authentik & trap "kill $$!" EXIT; \
	ENVIRONMENT="development" \
	GROUP_MAPPING_CONFIG="./config/group_mapping.yaml" \
	TOKEN_SECRET="secret" \
	AUTHENTIK_API_TOKEN="fake" \
	AUTHENTIK_URL="http://127.0.0.1:9100" \
	BADGE_SECRET="dev-badge-secret" \
	BADGE_DB="./data/badges.json" \
	PUBLIC_URL="http://localhost:3000" \
	go run cmd/multipass/main.go

# Install dependencies
install:
	npm install

# Build CSS with Tailwind
css: install
	npm run build:css

# Watch CSS changes
css-watch: install
	npm run watch:css

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf node_modules/
	rm -f web/static/css/tailwind.css
