build:
	go build -o bin/tempo cmd/tempo/*.go

static-build:
	CGO_ENABLED=1 go build -tags "static" -ldflags "-linkmode external -extldflags -static" -o bin/tempo cmd/tempo/*.go
