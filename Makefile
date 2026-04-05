.PHONY: run build generate clean dev

# Build the server
build:
	go build -o bin/grocer-ease ./cmd/server

# Generate templ templates (requires templ CLI: go install github.com/a-h/templ/cmd/templ@latest)
generate:
	templ generate

# Run the server
run:
	go run ./cmd/server

# Generate and run (for development)
dev: generate run

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f grocer-ease.db
