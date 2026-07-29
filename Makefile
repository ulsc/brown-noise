# Variables
PREFIX ?= /usr/local
BINDIR = $(PREFIX)/bin
BINARY = brown_noise
DESTDIR ?=

.PHONY: all build install uninstall clean

all: build

build:
	@echo "Building Go binary..."
	go build -o $(BINARY) .

install: build
	@echo "Installing $(BINARY) to $(DESTDIR)$(BINDIR)..."
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 $(BINARY) $(DESTDIR)$(BINDIR)/$(BINARY)
	@echo "Installed $(BINARY) to $(DESTDIR)$(BINDIR)/$(BINARY)"

uninstall:
	@echo "Uninstalling $(BINARY) from $(DESTDIR)$(BINDIR)..."
	-rm -f $(DESTDIR)$(BINDIR)/$(BINARY)
	@echo "Uninstalled."

clean:
	@echo "Cleaning build artifacts..."
	rm -f $(BINARY)
