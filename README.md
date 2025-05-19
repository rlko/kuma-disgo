# Kuma-DisGo

A Discord bot that displays Uptime Kuma service statuses in a Discord channel.

## Features

- Real-time service status monitoring from Uptime Kuma's API
- Minimal and detailed view options
- Automatic status updates
- Restricted to server owners
- Persistent storage of status message info using SQLite

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
2. Look at [config.yaml.example](config.yaml.example) for configuration details and create a `config.yaml` file with your settings.
3. Run the bot:
```bash
./kuma-disgo
```

You can specify a custom config file path using the `-c` or `--config` flag:
```bash
./kuma-disgo -c /path/to/your/config.yaml
```

## Commands

- `/status [view:minimal|detailed]` - Show service statuses (restricted to server owners)

## Status Indicators

- 🟢 Up
- 🔴 Down
- 🟡 Pending
- 🔵 Maintenance
- ⚪ Unknown

## Storage

The bot uses SQLite to store status message info, allowing for persistent storage across restarts. The database file (`status.db`) is created in the same directory as the `config.yaml` file.

## Demo

![Demo](screenshots/demo.gif)

## License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details. 
