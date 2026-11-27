.PHONY: run test vet build linux appimage web web-serve

# The web build's version: busts the browser cache of app.wasm and the
# catalog after each release.
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

run:
	go run ./cmd/app

test:
	go test ./...

vet:
	go vet ./...
	GOOS=js GOARCH=wasm go vet ./internal/... ./cmd/web

# The Windows program as released (icon, version, zip in dist/).
build:
	go run ./tools/buildexe

# Linux: build next to the repository's data/ and open the window.
linux:
	go build -o RagnarokOfBuilding ./cmd/app && ./RagnarokOfBuilding

# Linux: the AppImage as released (dist/RagnarokOfBuilding-<version>-x86_64.AppImage).
appimage:
	tools/appimage.sh

# The browser version: a static site in dist/web. app.wasm also goes out
# gzipped (the page unzips it), so the download is small on any host.
web:
	rm -rf dist/web && mkdir -p dist/web
	GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w -X main.version=$(VERSION)" -o dist/web/app.wasm ./cmd/web
	gzip -9 -k -n dist/web/app.wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" dist/web/
	sed 's/__VERSION__/$(VERSION)/g' web/index.html > dist/web/index.html
	cp cmd/app/icon.png dist/web/icon.png
	go run ./tools/websplit -in data/gen/catalog.json.gz -out dist/web
	cp -r web/icons dist/web/icons
	cp -r web/sprites dist/web/sprites
	touch dist/web/.nojekyll

# Serve dist/web on http://localhost:8080.
web-serve: web
	go run ./tools/webserve
