.PHONY: run build generate clean

# Generate templ files and build
build: generate
	go build -o bin/grocer-ease ./cmd/server

# Generate templ templates
generate:
	templ generate

# Generate and run
run: generate
	go run ./cmd/server

# Clean build artifacts
clean:
	rm -rf bin/
	rm -f grocer-ease.db
