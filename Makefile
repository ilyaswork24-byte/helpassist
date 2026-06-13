BINARY := helpassist
PKG    := .
DIST   := dist
LDFLAGS := -s -w

# Цели кросс-компиляции: работает на любом устройстве Ubuntu/Debian,
# включая ARM-одноплатники и телефоны (Termux/proot).
PLATFORMS := \
	linux/amd64 \
	linux/arm64 \
	linux/arm \
	darwin/amd64 \
	darwin/arm64

.PHONY: build install test vet clean release $(PLATFORMS)

build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) $(PKG)

install: build
	@mkdir -p $(HOME)/.local/bin
	@cp $(BINARY) $(HOME)/.local/bin/$(BINARY)
	@echo "Установлено в $(HOME)/.local/bin/$(BINARY)"
	@echo "Убедитесь, что $(HOME)/.local/bin в вашем PATH."

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf $(BINARY) $(DIST)

# release: собирает бинарники под все платформы в dist/
release: $(PLATFORMS)

$(PLATFORMS):
	@mkdir -p $(DIST)
	GOOS=$(word 1,$(subst /, ,$@)) GOARCH=$(word 2,$(subst /, ,$@)) \
		go build -ldflags "$(LDFLAGS)" \
		-o $(DIST)/$(BINARY)-$(word 1,$(subst /, ,$@))-$(word 2,$(subst /, ,$@)) $(PKG)
	@echo "собрано: $(DIST)/$(BINARY)-$(word 1,$(subst /, ,$@))-$(word 2,$(subst /, ,$@))"
