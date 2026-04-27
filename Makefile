.PHONY: build test install clean

BINARY_NAME = workflow-plugin-gitlab
INSTALL_DIR ?= data/plugins/$(BINARY_NAME)
GO_ENV = GOWORK=off GOPRIVATE=github.com/GoCodeAlone/*

build:
	$(GO_ENV) go build -o bin/$(BINARY_NAME) ./cmd/$(BINARY_NAME)

test:
	$(GO_ENV) go test ./... -v -race

install: build
	mkdir -p $(DESTDIR)$(INSTALL_DIR)
	cp bin/$(BINARY_NAME) $(DESTDIR)$(INSTALL_DIR)/
	cp plugin.json $(DESTDIR)$(INSTALL_DIR)/
	cp plugin.contracts.json $(DESTDIR)$(INSTALL_DIR)/

clean:
	rm -rf bin/
