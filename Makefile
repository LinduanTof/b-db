BINARY := dbrix
DIST := dist
LDFLAGS := -s -w

.PHONY: build run test vet tidy demo-db release clean

build:
	go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(BINARY) ./cmd/dbrix

run:
	go run ./cmd/dbrix

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

release:
	mkdir -p $(DIST)
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-amd64 ./cmd/dbrix
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(DIST)/$(BINARY)-linux-arm64 ./cmd/dbrix
	cd $(DIST) && sha256sum $(BINARY)-linux-* > checksums.txt

clean:
	rm -rf bin $(DIST)
