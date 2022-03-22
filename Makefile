
BINARY:=	mpd-slack-status

build: $(BINARY)

$(BINARY): ; go build --trimpath -ldflags "-w -s" -o $@

clean: ; rm -f $(BINARY)
