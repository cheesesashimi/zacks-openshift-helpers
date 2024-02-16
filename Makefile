# Define the directory where binaries will be output
OUTPUT_DIR := ./_output

# Find all directories under ./cmd and use them as binary names
BINARY_NAMES := $(notdir $(wildcard ./cmd/*))

# Define build target for each binary
.PHONY: all $(BINARY_NAMES)
all: $(BINARY_NAMES)
$(BINARY_NAMES):
	@echo "Building $@..."
	@mkdir -p $(OUTPUT_DIR)/$@
	@go build -o $(OUTPUT_DIR)/$@ ./cmd/$@

# Define target for running tests
.PHONY: test
test:
	@echo "Running tests..."
	@go test -v -shuffle=on -count=1 ./...

# Define target for running golangci-lint
.PHONY: lint
lint:
	@echo "Running golangci-lint..."
	@golangci-lint run ./...

# Define clean target to remove output directory
.PHONY: clean
clean:
	@echo "Cleaning up..."
	@rm -rf $(OUTPUT_DIR)

.PHONY: update-goreleaser
update-goreleaser-config:
	@echo "Updating goreleaser config..."
	./hack/update-goreleaser.py $(BINARY_NAMES)

.PHONY: validate-goreleaser
validate-goreleaser-config:
	@echo "Validating goreleaser config..."
	@goreleaser build --snapshot --clean --single-target
