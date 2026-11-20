.PHONY: run test vet build

run:
	go run ./cmd/app

test:
	go test ./...

vet:
	go vet ./...

# The Windows program as released (icon, version, zip in dist/).
build:
	go run ./tools/buildexe
