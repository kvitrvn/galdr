.PHONY: fmt test vet run build tidy clean

fmt:
	go fmt ./...

test:
	CGO_ENABLED=0 go test ./...

vet:
	CGO_ENABLED=0 go vet ./...

build:
	CGO_ENABLED=0 go build -o bin/galdr ./cmd/player

run:
	CGO_ENABLED=0 go run ./cmd/player

tidy:
	go mod tidy

clean:
	rm -rf bin/
