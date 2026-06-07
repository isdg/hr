.PHONY: build install test tidy clean

build:
	go build -o bin/hrb .

install:
	go install .

test:
	go test ./...

tidy:
	go mod tidy

clean:
	rm -rf bin
