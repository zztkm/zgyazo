# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

zgyazo is a Windows-specific Gyazo client that automatically uploads screenshots to Gyazo. It runs as a background service with two main components:
- **File Monitor Service**: Watches Snipping Tool directory for new screenshots and uploads them
- **Hotkey Service**: Registers Ctrl+Shift+C to launch Snipping Tool

## Commands

### Build
```bash
make build      # Builds with -H=windowsgui flag (no console window)
make install    # Installs to user's Go bin directory
```

### Development
```bash
go run .        # Run with console output for debugging
go test ./...   # Run tests (when added)
```

## Architecture

### Core Services
1. **gyazo_service.go**: 
   - Uses fsnotify to monitor screenshot directory
   - Implements OAuth2 authentication for Gyazo API
   - Handles multipart form uploads with retry logic
   - Opens uploaded URLs in browser

2. **shortcutkey_service.go**:
   - Registers Windows global hotkey using golang.org/x/sys/windows
   - Creates message-only window for hotkey events
   - Launches Snipping Tool via exec.Command

### Configuration
Located at `%APPDATA%/zgyazo/config.json`:
```json
{
  "gyazo_access_token": "YOUR_TOKEN",
  "snipping_tool_save_path": "C:\\Users\\USERNAME\\Pictures\\Snipping Tool"
}
```

### Key Implementation Details
- Services run concurrently using goroutines
- Retry logic handles Windows file locking (100ms delay, 10 attempts)
- Logging outputs to both file and stdout
- Built with `-H=windowsgui` to run without console

## Important Notes
- **Hardcoded Path**: gyazo_service.go:77 has hardcoded monitor path that should use config
- **No Graceful Shutdown**: Currently missing (implementation proposal in docs/development.md)
- **Windows-Only**: Uses Windows-specific APIs for hotkeys
- **Upload Settings**: access_policy and metadata_is_public are hardcoded in gyazo_service.go