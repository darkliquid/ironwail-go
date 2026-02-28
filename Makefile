QUAKE_DIR ?= /home/darkliquid/Games/Heroic/Quake
TIMEOUT ?= 5
GO_RUN := go run -tags=gogpu ./cmd/ironwailgo -basedir $(QUAKE_DIR)

.PHONY: smoke-menu smoke-headless smoke-map-start smoke-all test clean-logs

smoke-menu: clean-logs
	WAYLAND_DISPLAY= timeout $(TIMEOUT) $(GO_RUN) 2>&1 | tee smoke-menu.log || true
	grep "FS mounted" smoke-menu.log
	grep "QC loaded" smoke-menu.log
	grep "menu active" smoke-menu.log
	grep "frame loop started" smoke-menu.log

smoke-headless: clean-logs
	WAYLAND_DISPLAY= timeout $(TIMEOUT) $(GO_RUN) -headless 2>&1 | tee smoke-headless.log || true
	grep "FS mounted" smoke-headless.log
	grep "QC loaded" smoke-headless.log
	grep "menu active" smoke-headless.log
	grep "frame loop started" smoke-headless.log

smoke-map-start: clean-logs
	WAYLAND_DISPLAY= timeout $(TIMEOUT) $(GO_RUN) start 2>&1 | tee smoke-map-start.log || true
	grep "FS mounted" smoke-map-start.log
	grep "QC loaded" smoke-map-start.log
	grep "menu active" smoke-map-start.log
	grep "map spawn started" smoke-map-start.log
	grep "map spawn finished" smoke-map-start.log
	grep "frame loop started" smoke-map-start.log

smoke-all: smoke-menu smoke-headless smoke-map-start

test:
	QUAKE_DIR=$(QUAKE_DIR) go test ./internal/testutil

clean-logs:
	rm -f smoke-*.log
