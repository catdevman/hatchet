.PHONY: build test release clean compare browser-install

build:
	mise run build

test:
	mise run test

release:
	mise run release

clean:
	mise run clean

compare:
	@if [ -z "$(TARGET)" ]; then \
		echo "Usage: make compare TARGET=<url>"; \
		exit 1; \
	fi
	mise run compare $(TARGET)

browser-install:
	mise run browser-install
