# Package mods

## Purpose

The `mods` package implements a background downloader for game addons. It allows players to browse a remote manifest of mods, download them via HTTP, and install them into the local game directory. It is modeled after the addon subsystem found in modern Quake source ports.

## Key Types & Interfaces

- **`Downloader`**: The main runtime that manages manifest fetching and active installations.
- **`Manifest`**: The decoded representation of the `content.json` file provided by the addon server.
- **`RemoteMod`**: Describes a single mod available for download, including its name, author, and download URL.
- **`InstallState`**: A thread-safe structure that tracks the progress (bytes downloaded, status) of an ongoing installation.

## Core Workflow

1.  **Manifest Fetching**: `FetchManifest` retrieves the `content.json` file. It includes a caching mechanism that respects the `ManifestRetention` window and validates the server URL hasn't changed.
2.  **Installation**: When a mod is selected, `StartInstall` kicks off a background goroutine.
3.  **Downloading**: The installer streams the mod's `.pak` archive from the server to a temporary file (`pak0.download.tmp`).
4.  **Finalization**: Once the download is complete, the temporary file is atomically renamed to `pak0.pak`, and the `InstallState` is updated to `StatusInstalled`.

## Integration

- **`internal/menu`**: Provides the UI for browsing the manifest and triggering downloads.
- **`internal/host`**: Typically owns the `Downloader` instance and provides the necessary configuration (install directory, cache directory).
- **`internal/cvar`**: The addon server URL is usually configurable via an engine cvar.

## Learning Tips

- **Atomic File Operations**: Notice how the downloader uses a `.tmp` file and `os.Rename` to ensure that a partial download never results in a corrupted mod installation.
- **Concurrency**: Observe the use of `atomic.Int32` and `sync.Mutex` in `InstallState` and `Downloader` to allow the main engine thread to safely query progress from a background download goroutine.
- **HTTP Caching**: Examine `FetchManifest` to see how it uses `addons.url.dat` to invalidate the cache if the user switches to a different addon server.
