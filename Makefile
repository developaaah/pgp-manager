BIN_NAME     := pgpmanager
RELEASE_NAME := PGP-Manager
BUNDLE       := PGP Manager.app
BINDIR       ?= ./bin
WAILS        := wails
DOCKER       := docker
GO_VERSION   := 1.26

VERSION      ?= $(shell git describe --tags --always 2>/dev/null || echo "dev")
GO_LDFLAGS   := -X main.appVersion=$(VERSION)

# Apple bundles and Windows PE resources require bare semver ("1.2.3"), not "v1.2.3".
PLIST_VERSION := $(patsubst v%,%,$(VERSION))

.PHONY: all release clean dev frontend-dist patch-wails-version \
        darwin darwin-arm64 darwin-amd64 \
        dmg dmg-arm64 dmg-amd64 \
        linux linux-amd64 linux-arm64 linux-386 \
        pkg pkg-amd64 pkg-arm64 pkg-386 \
        windows windows-amd64 windows-386

release: dmg-arm64 dmg-amd64 pkg-amd64 pkg-arm64 pkg-386 windows-amd64 windows-386

all: release

dev:
	$(WAILS) dev

clean:
	rm -rf $(BINDIR) build/bin


# macOS
darwin: darwin-arm64

darwin-arm64:
	/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $(PLIST_VERSION)" build/darwin/Info.plist
	/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $(PLIST_VERSION)" build/darwin/Info.plist
	$(WAILS) build -clean -platform darwin/arm64 -ldflags "$(GO_LDFLAGS)"
	@mkdir -p "$(BINDIR)/darwin-arm64"
	rm -rf "$(BINDIR)/darwin-arm64/$(BUNDLE)"
	mv "build/bin/$(BUNDLE)" "$(BINDIR)/darwin-arm64/"

darwin-amd64:
	/usr/libexec/PlistBuddy -c "Set :CFBundleShortVersionString $(PLIST_VERSION)" build/darwin/Info.plist
	/usr/libexec/PlistBuddy -c "Set :CFBundleVersion $(PLIST_VERSION)" build/darwin/Info.plist
	$(WAILS) build -clean -platform darwin/amd64 -ldflags "$(GO_LDFLAGS)"
	@mkdir -p "$(BINDIR)/darwin-amd64"
	rm -rf "$(BINDIR)/darwin-amd64/$(BUNDLE)"
	mv "build/bin/$(BUNDLE)" "$(BINDIR)/darwin-amd64/"


# macOS release DMGs
dmg: dmg-arm64 dmg-amd64

dmg-arm64: darwin-arm64
	build/dmg/create-dmg.sh "$(BINDIR)/darwin-arm64/$(BUNDLE)" "$(BINDIR)/$(RELEASE_NAME)_mac_silicon_$(VERSION).dmg"

dmg-amd64: darwin-amd64
	build/dmg/create-dmg.sh "$(BINDIR)/darwin-amd64/$(BUNDLE)" "$(BINDIR)/$(RELEASE_NAME)_mac_intel_$(VERSION).dmg"


# Windows
patch-wails-version:
	python3 -c "import json; f='wails.json'; d=json.load(open(f)); d['info']['productVersion']='$(PLIST_VERSION)'; open(f,'w').write(json.dumps(d,indent=2,ensure_ascii=False)+'\n')"

windows: windows-amd64 windows-386

windows-amd64: patch-wails-version
	$(WAILS) build -clean -platform windows/amd64 -ldflags "$(GO_LDFLAGS)"
	@mkdir -p $(BINDIR)
	mv build/bin/$(BIN_NAME)*.exe "$(BINDIR)/$(RELEASE_NAME)_windows_64bit_$(VERSION).exe"

windows-386: patch-wails-version
	$(WAILS) build -clean -platform windows/386 -ldflags "$(GO_LDFLAGS)"
	@mkdir -p $(BINDIR)
	mv build/bin/$(BIN_NAME)*.exe "$(BINDIR)/$(RELEASE_NAME)_windows_32bit_$(VERSION).exe"


# Linux (Docker)
docker_platform = linux/$(if $(filter arm,$(1)),arm/v7,$(1))

# $(1) = output filename  $(2) = docker/Go arch
define build_linux
	$(DOCKER) build --platform $(call docker_platform,$(2)) \
		--build-arg GO_VERSION=$(GO_VERSION) \
		-t $(BIN_NAME)-linux-builder-$(2) \
		-f build/docker/Dockerfile.linux build/docker
	@mkdir -p $(BINDIR)
	$(DOCKER) run --rm --platform $(call docker_platform,$(2)) \
		-v "$(CURDIR)":/src -w /src \
		-v $(BIN_NAME)-go-build-$(2):/root/.cache/go-build \
		-v $(BIN_NAME)-go-mod:/go/pkg/mod \
		$(BIN_NAME)-linux-builder-$(2) \
		go build -trimpath -buildvcs=false -tags desktop,production,webkit2_41 \
			-ldflags "-w -s $(GO_LDFLAGS)" -o $(BINDIR)/$(1) .
endef

linux: linux-amd64 linux-arm64 linux-386

linux-amd64: frontend-dist
	$(call build_linux,$(RELEASE_NAME)_linux_amd_$(VERSION),amd64)

linux-arm64: frontend-dist
	$(call build_linux,$(RELEASE_NAME)_linux_arm_$(VERSION),arm64)

linux-386: frontend-dist
	$(call build_linux,$(RELEASE_NAME)_linux_32bit_$(VERSION),386)

frontend-dist:
	cd frontend && yarn install --frozen-lockfile && yarn build


# Linux packages (.deb + .rpm)
pkg: pkg-amd64 pkg-arm64 pkg-386

pkg-amd64: linux-amd64
	build/linux/package.sh "$(BINDIR)/$(RELEASE_NAME)_linux_amd_$(VERSION)" amd64 $(VERSION) $(BINDIR)

pkg-arm64: linux-arm64
	build/linux/package.sh "$(BINDIR)/$(RELEASE_NAME)_linux_arm_$(VERSION)" arm64 $(VERSION) $(BINDIR)

pkg-386: linux-386
	build/linux/package.sh "$(BINDIR)/$(RELEASE_NAME)_linux_32bit_$(VERSION)" 386 $(VERSION) $(BINDIR)
