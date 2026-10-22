BINARY := bdb

.PHONY: build run test vet tidy demo-db clean

build:
	go build -trimpath -ldflags "-s -w" -o bin/$(BINARY) ./cmd/bdb

run:
	go run ./cmd/bdb

test: demo-db
	go test ./...

vet:
	go vet ./...

tidy:
	go mod tidy

demo-db:
	mkdir -p testdata
	rm -f testdata/demo.db
	sqlite3 testdata/demo.db < testdata/schema.sql

clean:
	rm -rf bin
