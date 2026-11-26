.PHONY: run test vet build linux appimage

run:
	go run ./cmd/app

test:
	go test ./...

vet:
	go vet ./...

# The Windows program as released (icon, version, zip in dist/).
build:
	go run ./tools/buildexe

# Linux: build next to the repository's data/ and open the window.
linux:
	go build -o RagnarokOfBuilding ./cmd/app && ./RagnarokOfBuilding

# Linux: the AppImage as released (dist/RagnarokOfBuilding-<version>-x86_64.AppImage).
appimage:
	tools/appimage.sh
