.PHONY: build test lint clean

build:
	go build -o pushbadger ./cmd/pushbadger

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f pushbadger
