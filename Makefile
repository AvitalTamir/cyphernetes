# Define the binary name
BINARY_NAME=cyphernetes

# Define the default make target
all: bt

# Build then Test
bt: build test

# Define how to build the project
build: gen-parser
	@echo "👷 Building ${BINARY_NAME}..."
	(cd cmd/cyphernetes && go build -o ${BINARY_NAME} > /dev/null)
	mkdir -p dist/
	mv cmd/cyphernetes/${BINARY_NAME} dist/
	@echo "🎉 Done!"

# Define how to run tests
test:
	@echo "🧪 Running tests..."
	go test ./... | sed 's/^/   /g'

# Define how to generate the grammar parser
gen-parser:
	@echo "🧠 Generating parser..."
	goyacc -o pkg/parser/cyphernetes.go -p "yy" grammar/cyphernetes.y

# Define how to clean the build
clean:
	@echo "🫧 Cleaning..."
	go clean -cache > /dev/null
	rm -rf dist/

# Define a phony target for the clean command to ensure it always runs
.PHONY: clean
.SILENT: build test gen-parser clean

# Add a help command to list available targets
help:
	@echo "Available commands:"
	@echo "  all          - Build the project."
	@echo "  build        - Compile the project into a binary."
	@echo "  test         - Run tests."
	@echo "  gen-parser   - Generate the grammar parser using Pigeon."
	@echo "  clean        - Remove binary and clean up."
