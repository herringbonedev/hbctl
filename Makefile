BINARY ?= hbctl
VERSION ?= alpha-0.6.0
REV ?= rev-$(shell openssl rand -hex 8 2>/dev/null || python3 -c 'import secrets; print(secrets.token_hex(8))')
LDFLAGS := -s -w -X github.com/herringbonedev/hbctl/cmd.Version=$(VERSION) -X github.com/herringbonedev/hbctl/cmd.Revision=$(REV)

.PHONY: build
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) .

.PHONY: install
install: build
	sudo cp $(BINARY) /usr/local/bin/$(BINARY)
