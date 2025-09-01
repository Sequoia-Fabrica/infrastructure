.PHONY: build run dev css css-watch install

# Build the Go application
build:
	go build -o bin/multipass cmd/multipass/main.go

# Run the application
run: build
	./bin/multipass

# Run in development mode
dev: css
	$(info Running in development mode...)
	GROUP_MAPPING_CONFIG="./config/group_mapping.yaml" \
	TOKEN_SECRET="secret" \
	AUTHENTIK_API_TOKEN="example" \
	AUTHENTIK_URL="https://login.sequoia.garden" \
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
