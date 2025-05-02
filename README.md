# Kuma-DisGo

A Discord bot that displays Uptime Kuma service statuses in a Discord channel.

## Features

- Real-time service status monitoring from Uptime Kuma's API
- Minimal and detailed view options
- Automatic status updates
- Restricted to server owners and administrators

## Building

The project uses a Makefile for building and packaging:

```bash
# Build the binary
make build

# Build with compression (requires UPX)
make build COMPRESS=1

# Create a Debian package
make deb

# Create a tarball
make tarball

# Install system-wide
make install

# Install for current user
make user-install
```

The binary will be created in the project directory, while packages and archives will be placed in the `build` directory.

## Setup

1. Create a Discord bot and get its token
2. Create a `config.yaml` file with your settings:

```yaml
discord:
  token: "your-discord-bot-token"

uptime_kuma:
  base_url: "https://your-uptime-kuma-instance"
  api_key: "your-uptime-kuma-api-key"

update_interval: "60s"  # Update interval (default: 60s)

sections:
  - name: "Production"
    services:
      - name: "api"
        display_name: "API Server"  # Optional display name
      - name: "cache"
        display_name: "Cache Server"
  - name: "Development"
    services:
      - name: "dev-api"
        display_name: "Dev API"
```

3. Run the bot:
```bash
./kuma-disgo
```

## Commands

- `/status [view:minimal|detailed]` - Show service statuses (restricted to server owners and administrators)

## Status Indicators

- 🟢 Up
- 🔴 Down
- 🟡 Pending
- 🔵 Maintenance
- ⚪ Unknown

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 
