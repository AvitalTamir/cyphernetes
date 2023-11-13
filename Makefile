# Define the binary name
BINARY_NAME=cyphernetes

# Define the default make target
all: test build

# Define how to build the project
build: gen-parser
	@echo "👷 Building ${BINARY_NAME}..."
	go build -o ${BINARY_NAME} main.go > /dev/null
	@echo "🎉 Done!"

# Define how to run tests
test:
	@echo "🧪 Running tests..."
	go test ./... | sed 's/^/   /g'

# Define how to generate the grammar parser
gen-parser:
	@echo "🧠 Generating parser..."
#	pigeon -o cmd/cyphernetes.go grammer/cyphernetes.peg > /dev/null
	goyacc -o cmd/cyphernetes.go -p "yy" grammer/cyphernetes.y

# Define how to clean the build
clean:
	@echo "🫧 Cleaning..."
	go clean > /dev/null
	rm ${BINARY_NAME} > /dev/null

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
