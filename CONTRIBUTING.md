# Contributing to SensorPanel

Thank you for your interest in contributing!

## Development Setup

1. Prerequisites:
   - Go 1.21+
   - Node.js 18+ (for theme development)
   - libusb (Linux: `libusb-1.0-0-dev`, macOS: `brew install libusb`)

2. Clone and build:
   ```bash
   git clone https://github.com/alperen/sensorpanel.git
   cd sensorpanel
   go build .
   ```

3. Run with Mage:
   ```bash
   go run magefiles/magefile.go build  # Build
   go run magefiles/magefile.go test   # Run tests
   go run magefiles/magefile.go lint   # Run linter
   ```

   Or install mage globally:
   ```bash
   go install github.com/magefile/mage@latest
   mage build
   mage -l  # List all targets
   ```

## Adding Support for a New Device

The easiest way to add a new device is using the interactive wizard:

```bash
./sensorpanel device create
```

This will prompt you for device information and generate a skeleton profile.

### Manual Process

1. Create a new file `pkg/device/yourdevice.go`

2. Implement the `DeviceProfile` interface:
   - `ID()`, `Name()`, `Description()` - Device identity
   - `VendorID()`, `ProductID()`, `Matches()` - USB identification
   - `Width()`, `Height()`, `ColorFormat()`, `ByteOrder()` - Display properties
   - `BlitCommand()` - Build command bytes for sending image data
   - `BacklightCommand()` - Build command for backlight control
   - `ConvertImage()` - Convert image to device's pixel format

3. Register your profile in `pkg/device/registry.go`

4. See `docs/adding-devices.md` for protocol research tips.

5. Test with your device and submit a PR!

## Creating Themes

1. Create a new theme:
   ```bash
   ./sensorpanel theme create my-theme
   ```

2. Develop with hot reload:
   ```bash
   ./sensorpanel theme dev my-theme
   ```

3. The theme uses React + TypeScript with a pre-built SDK in `lib/sensorpanel/`.

4. See `docs/creating-themes.md` for detailed documentation.

## Code Style

- Run `go fmt` before committing
- Run `golangci-lint run` to check for issues
- Follow existing code patterns
- Add tests for new functionality

## Pull Request Guidelines

1. Create a feature branch from `main`
2. Make your changes with clear commit messages
3. Run checks: `mage check` or:
   ```bash
   go vet ./...
   go test ./...
   golangci-lint run
   ```
4. Submit PR with clear description of changes

## Reporting Issues

- Use the appropriate issue template
- Include device info (VID/PID, lsusb output) for hardware issues
- Include logs and error messages
- Include steps to reproduce

## Questions?

Feel free to open a discussion or issue if you have questions about contributing.
