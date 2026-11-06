APP      := task-cli
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
COMMIT   := $(shell git rev-parse --short HEAD 2>/dev/null || echo none)
DATE     := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS  := -s -w -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)
BUILD    := CGO_ENABLED=0 go build -trimpath -ldflags="$(LDFLAGS)"

PREFIX   ?= $(HOME)/.local
BINDIR   ?= $(PREFIX)/bin
SKILLDIR ?= $(HOME)/.agents/skills/$(APP)

.PHONY: build build-all clean test checksums run install uninstall install-skill

build:
	@mkdir -p dist
	$(BUILD) -o dist/$(APP) .

build-all: clean
	@mkdir -p dist
	GOOS=linux GOARCH=amd64         $(BUILD) -o dist/$(APP)-linux-amd64 .
	GOOS=linux GOARCH=arm64         $(BUILD) -o dist/$(APP)-linux-arm64 .
	GOOS=linux GOARCH=arm GOARM=7   $(BUILD) -o dist/$(APP)-linux-armv7 .

checksums:
	cd dist && sha256sum $(APP)-* > checksums.txt

install: build
	@mkdir -p $(BINDIR)
	install -m 755 dist/$(APP) $(BINDIR)/$(APP)
	@echo "Instalado com sucesso em $(BINDIR)/$(APP)"

uninstall:
	rm -f $(BINDIR)/$(APP)
	@echo "Removido $(BINDIR)/$(APP)"

install-skill:
	@mkdir -p $(SKILLDIR)
	install -m 644 SKILL.md $(SKILLDIR)/SKILL.md
	@echo "Skill instalada/atualizada em $(SKILLDIR)/SKILL.md"

test:
	go test ./...

run:
	go run .

clean:
	rm -rf dist $(APP)
