
BINARY:=	music-status
SRC!=		find . -type f -name '*.go'

build: $(BINARY)

$(BINARY): $(SRC) go.mod
	go build --trimpath -ldflags "-w -s" -o $@ ./cmd/

clean: ; rm -f $(BINARY)
