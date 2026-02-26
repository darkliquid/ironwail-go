.PHONY: smoke-menu smoke-headless smoke-map-start test

smoke-menu:
	WAYLAND_DISPLAY= timeout 5 go run -tags=gogpu ./cmd/ironwailgo || true

smoke-headless:
	WAYLAND_DISPLAY= timeout 5 go run -tags=gogpu ./cmd/ironwailgo -headless || true

smoke-map-start:
	WAYLAND_DISPLAY= timeout 5 go run -tags=gogpu ./cmd/ironwailgo start || true

test:
	QUAKE_DIR=/home/darkliquid/Games/Heroic/Quake go test ./internal/testutil
