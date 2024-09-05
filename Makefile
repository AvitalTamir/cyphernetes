# Define the binary name
BINARY_NAME=cyphernetes
TARGET_KERNELS=darwin linux
TARGET_ARCHS=amd64 arm64
# Define the default make target
all: operator bt

# Build then Test
bt: build test

# Define how to build the project
build: gen-parser
	@echo "👷 Building ${BINARY_NAME}..."
	(cd cmd/cyphernetes && go build -o ${BINARY_NAME} > /dev/null)
	mkdir -p dist/
	mv cmd/cyphernetes/${BINARY_NAME} dist/cyphernetes-darwin-arm64
	@echo "🎉 Done!"

build-all-platforms-and-archs:
	@echo "👷 Building ${BINARY_NAME}..."
	@for kernel in $(TARGET_KERNELS); do \
		for arch in $(TARGET_ARCHS); do \
			echo "   - $$kernel/$$arch"; \
			cd cmd/cyphernetes && GOOS=$$kernel GOARCH=$$arch go build -o ${BINARY_NAME} > /dev/null; \
			mkdir -p ../../dist/; \
			mv ${BINARY_NAME} ../../dist/cyphernetes-$$kernel-$$arch; \
			cd ../..; \
		done; \
	done
	@echo "🎉 Done!"

# Define how to run tests
test:
	@echo "🧪 Running tests..."
	go test ./... | sed 's/^/   /g'

# Define how to generate the grammar parser
gen-parser:
	@echo "🧠 Generating parser..."
	goyacc -o pkg/parser/cyphernetes.go -p "yy" grammar/cyphernetes.y

operator:
	@echo "🤖 Creating operator manifests..."
	$(MAKE) -C operator deployment-manifests > /dev/null

.PHONY: operator

# Define how to clean the build
clean:
	@echo "💧 Cleaning..."
	go clean -cache > /dev/null
	rm -rf dist/
	rm -rf coverage.out
	rm -rf cmd/cyphernetes/manifests

coverage:
	mkdir -p .coverage
	@echo "🧪 Generating coverage report for cmd/cyphernetes..."
	go test ./cmd/cyphernetes -coverprofile=.coverage/coverage.out
	go tool cover -func=.coverage/coverage.out | sed 's/^/   /g'
	go tool cover -html=.coverage/coverage.out -o .coverage/coverage.html
	@echo "🌎 Opening coverage report in browser..."
	open file://$$(pwd)/.coverage/coverage.html

# Define a phony target for the clean command to ensure it always runs
.PHONY: clean
.SILENT: build test gen-parser clean coverage operator

# Add a help command to list available targets
help:
	@echo "Available commands:"
	@echo "  all          - Build the project."
	@echo "  build        - Compile the project into a binary."
	@echo "  test         - Run tests."
	@echo "  gen-parser   - Generate the grammar parser using Pigeon."
	@echo "  clean        - Remove binary and clean up."
