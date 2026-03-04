VERSION ?= $(shell grep 'const version' main.go | head -1 | cut -d'"' -f2)
MODULE  := github.com/peregrine-digital/activate-framework/cli
LDFLAGS := -s -w

# All target platforms
PLATFORMS := darwin-arm64 darwin-amd64 linux-arm64 linux-amd64 windows-amd64

# npm platform directory mapping (Go uses amd64, npm uses x64)
npm_platform = $(subst amd64,x64,$(subst windows,win32,$1))

.PHONY: all clean build-all npm-stage

all: build-all

# Build for a single platform: make build GOOS=darwin GOARCH=arm64
build:
	go build -ldflags "$(LDFLAGS)" -o activate .

# Build all platforms into dist/
build-all: $(PLATFORMS)

$(PLATFORMS):
	$(eval OS := $(word 1,$(subst -, ,$@)))
	$(eval ARCH := $(word 2,$(subst -, ,$@)))
	$(eval EXT := $(if $(filter windows,$(OS)),.exe,))
	@echo "Building $(OS)/$(ARCH)..."
	@mkdir -p dist/$@
	GOOS=$(OS) GOARCH=$(ARCH) go build -ldflags "$(LDFLAGS)" -o dist/$@/activate$(EXT) .

# Copy binaries into npm platform package directories for publishing
npm-stage: build-all
	@echo "Staging binaries into npm platform packages..."
	@for plat in $(PLATFORMS); do \
		npm_dir=$$(echo "$$plat" | sed 's/amd64/x64/' | sed 's/windows/win32/'); \
		ext=""; \
		echo "$$plat" | grep -q windows && ext=".exe"; \
		cp dist/$$plat/activate$$ext npm/platforms/$$npm_dir/activate$$ext; \
		echo "  ✓ $$npm_dir"; \
	done

# Publish all npm packages (run npm-stage first)
npm-publish: npm-stage
	@echo "Publishing platform packages..."
	@for dir in npm/platforms/*/; do \
		echo "  Publishing $$(basename $$dir)..."; \
		cd $$dir && npm publish --access public && cd -; \
	done
	@echo "Publishing main package..."
	cd npm && npm publish --access public

clean:
	rm -rf dist/
	rm -f activate
	@for dir in npm/platforms/*/; do \
		rm -f $$dir/activate $$dir/activate.exe; \
	done

# Show version
version:
	@echo $(VERSION)
