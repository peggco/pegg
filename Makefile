WAILS ?= wails
DESKTOP_TAGS ?= gui $(if $(filter Linux,$(shell uname -s)),webkit2_41)
ICON_HASH := $(shell sha256sum pkg/desktop/assets/appicon.png 2>/dev/null | cut -c1-8)
ICON_NAME := pegg-$(if $(ICON_HASH),$(ICON_HASH),latest)

vet:
	go vet ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run

test:
	go test -v ./...

build:
	go build -o bin/pegg cmd/pegg/main.go

run:
	./bin/pegg

clean:
	rm -rf bin pkg/desktop/build/bin pkg/desktop/frontend/dist

desktop-deps:
	npm --prefix pkg/desktop/frontend install --no-audit --no-fund

desktop-frontend:
	npm --prefix pkg/desktop/frontend run build

desktop-icon:
	mkdir -p pkg/desktop/build
	cp pkg/desktop/assets/appicon.png pkg/desktop/build/appicon.png

desktop-dev: desktop-deps desktop-icon
	cd pkg/desktop && $(WAILS) dev -tags "$(DESKTOP_TAGS)"

desktop-build: desktop-deps desktop-icon
	cd pkg/desktop && $(WAILS) build -tags "$(DESKTOP_TAGS)"

desktop-test: desktop-frontend
	npm --prefix pkg/desktop/frontend run typecheck
	npm --prefix pkg/desktop/frontend run test
	go test -tags "$(DESKTOP_TAGS)" ./pkg/desktop/...

desktop-install: desktop-build
	mkdir -p $(HOME)/.local/bin $(HOME)/.local/share/applications $(HOME)/.local/share/icons/hicolor/512x512/apps
	cp pkg/desktop/build/bin/pegg-desktop $(HOME)/.local/bin/pegg-desktop.tmp
	chmod 755 $(HOME)/.local/bin/pegg-desktop.tmp
	mv -f $(HOME)/.local/bin/pegg-desktop.tmp $(HOME)/.local/bin/pegg-desktop
	cp pkg/desktop/assets/appicon.png $(HOME)/.local/share/icons/hicolor/512x512/apps/$(ICON_NAME).png
	cp pkg/desktop/assets/appicon.png $(HOME)/.local/share/icons/hicolor/512x512/apps/pegg.png
	sed 's|^Icon=.*|Icon=$(ICON_NAME)|' pkg/desktop/assets/pegg.desktop > $(HOME)/.local/share/applications/pegg.desktop
	@command -v update-desktop-database >/dev/null 2>&1 && update-desktop-database $(HOME)/.local/share/applications || true
	@command -v gtk-update-icon-cache >/dev/null 2>&1 && gtk-update-icon-cache -f -t $(HOME)/.local/share/icons/hicolor || true
	@command -v kbuildsycoca6 >/dev/null 2>&1 && kbuildsycoca6 --noincremental >/dev/null 2>&1 || true
	@command -v kbuildsycoca5 >/dev/null 2>&1 && kbuildsycoca5 --noincremental >/dev/null 2>&1 || true

.PHONY: vet fmt lint test build run clean desktop-deps desktop-frontend desktop-icon desktop-dev desktop-build desktop-test desktop-install
