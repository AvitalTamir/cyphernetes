# Define the binary name
BINARY_NAME=cyphernetes
KUBECTL_PLUGIN_NAME=kubectl-cypher
TARGET_KERNELS=darwin linux windows
TARGET_ARCHS=amd64 arm64
VERSION ?= dev

# Define the default make target
all: operator-manifests bt
	@echo "🎉 Done!"

# Build then Test
bt: build test

# Define how to build the project
build: web-build notebook-build
	@echo "👷 Building ${BINARY_NAME}..."
	(cd cmd/cyphernetes && go build -o ${BINARY_NAME} -ldflags "-X main.Version=${VERSION}" > /dev/null)
	mkdir -p dist/
	mv cmd/cyphernetes/${BINARY_NAME} dist/cyphernetes

# Build the lean kubectl plugin
build-kubectl-plugin:
	@echo "👷 Building ${KUBECTL_PLUGIN_NAME}..."
	(cd cmd/kubectl-cypher && go build -o ${KUBECTL_PLUGIN_NAME} -ldflags "-X main.Version=${VERSION}" > /dev/null)
	mkdir -p dist/
	mv cmd/kubectl-cypher/${KUBECTL_PLUGIN_NAME} dist/kubectl-cypher

build-all-platforms:
	@echo "👷 Building ${BINARY_NAME}..."
	@for kernel in $(TARGET_KERNELS); do \
		for arch in $(TARGET_ARCHS); do \
			echo "   - $$kernel/$$arch"; \
			cd cmd/cyphernetes && GOOS=$$kernel GOARCH=$$arch go build -o ${BINARY_NAME} -ldflags "-X main.Version=${VERSION}" > /dev/null; \
			mkdir -p ../../dist/; \
			mv ${BINARY_NAME} ../../dist/cyphernetes-$$kernel-$$arch; \
			cd ../..; \
		done; \
	done
	@echo "🎉 Done!"

build-kubectl-plugin-all-platforms:
	@echo "👷 Building ${KUBECTL_PLUGIN_NAME} for all platforms..."
	@for kernel in $(TARGET_KERNELS); do \
		for arch in $(TARGET_ARCHS); do \
			echo "   - $$kernel/$$arch"; \
			cd cmd/kubectl-cypher && GOOS=$$kernel GOARCH=$$arch go build -o ${KUBECTL_PLUGIN_NAME} -ldflags "-X main.Version=${VERSION}" > /dev/null; \
			mkdir -p ../../dist/; \
			mv ${KUBECTL_PLUGIN_NAME} ../../dist/kubectl-cypher-$$kernel-$$arch; \
			cd ../..; \
		done; \
	done
	@echo "🎉 Done!"

test:
	@echo "🧪 Running tests..."
	go test ./...

.PHONY: test-e2e
test-e2e:
	go install github.com/onsi/ginkgo/v2/ginkgo@latest
	ginkgo -v ./pkg/core/e2e

operator-manifests:
	@echo "🤖 Creating operator manifests..."
	$(MAKE) -C operator deployment-manifests > /dev/null

operator-docker-build:
	@echo "🐳 Building operator docker image..."
	$(MAKE) -C operator docker-build IMG=fatliverfreddy/cyphernetes-operator:latest > /dev/null

operator-docker-push:
	@echo "🐳 Pushing operator docker image..."
	$(MAKE) -C operator docker-push IMG=fatliverfreddy/cyphernetes-operator:latest > /dev/null

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
	go test ./... -coverprofile=.coverage/coverage.out
	go tool cover -func=.coverage/coverage.out | sed 's/^/   /g'
	go tool cover -html=.coverage/coverage.out -o .coverage/coverage.html
	@echo "🌎 Opening coverage report in browser..."
	open file://$$(pwd)/.coverage/coverage.html

operator-test:
	@echo "🤖 Testing operator..."
	$(MAKE) -C operator test
	$(MAKE) -C operator test-e2e

web-build:
	@echo "🌐 Building web interface..."
	cd web && pnpm install > /dev/null && pnpm run build > /dev/null
	@echo "📦 Copying web artifacts..."
	rm -rf cmd/cyphernetes/web
	cp -r web/dist cmd/cyphernetes/web

web-test:
	@echo "🧪 Running web tests..."
	cd web && pnpm install && pnpm test

notebook-build:
	@echo "📓 Building notebook interface..."
	cd notebook && pnpm install > /dev/null && pnpm run build > /dev/null
	@echo "📦 Copying notebook artifacts..."
	rm -rf pkg/notebook/static
	cp -r notebook/dist pkg/notebook/static

notebook-test:
	@echo "🧪 Running notebook tests..."
	cd notebook && pnpm install && pnpm test

web-run: build
	./dist/cyphernetes web

# Define a phony target for the clean command to ensure it always runs
.PHONY: clean build-kubectl-plugin build-kubectl-plugin-all-platforms
.SILENT: build build-kubectl-plugin test gen-parser clean coverage operator operator-test operator-manifests operator-docker-build operator-docker-push web-build web-test notebook-build notebook-test

# Add a help command to list available targets
help:
	@echo "Available commands:"
	@echo "  all                                - Build the project."
	@echo "  build                              - Compile the main project into a binary."
	@echo "  build-kubectl-plugin               - Build the lean kubectl-cypher plugin."
	@echo "  build-all-platforms                - Build main binary for all platforms."
	@echo "  build-kubectl-plugin-all-platforms - Build kubectl plugin for all platforms."
	@echo "  test                               - Run tests."
	@echo "  clean                              - Remove binaries and clean up."
