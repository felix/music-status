
BINARY:=	music-status
SRC!=		find . -type f -name '*.go'

build: $(BINARY)

$(BINARY): $(SRC) go.mod
	go build --trimpath -ldflags "-w -s" -o $@ ./cmd/

test: lint
	go test -race -short -coverprofile=coverage.out -covermode=atomic ./...

cover: test
	go tool cover -func=coverage.out

lint:
	test -n "$$(command -v golangci-lint)" && golangci-lint run ./... || go vet ./...

clean:
	rm -f $(BINARY)
	rm -f coverage.*
