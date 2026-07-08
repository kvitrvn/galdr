.PHONY: fmt test vet run build tidy clean

fmt:
	go fmt ./...

test:
	go test ./...

vet:
	go vet ./...

build:
	go build -o bin/galdr ./cmd/player

run:
	go run ./cmd/player

tidy:
	go mod tidy

clean:
	rm -rf bin/