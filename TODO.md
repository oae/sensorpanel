# SensorPanel Refactoring TODO

## Overview

Refactor sensorpanel for:
1. Modular device support (different USB displays)
2. Streamlined theme development (single command, TypeScript SDK)
3. Cross-platform build system (Mage)
4. Open source readiness

---

## Phase 1: Device Profile System

### New Package: `pkg/device/`

| File | Purpose |
|------|---------|
| `profile.go` | DeviceProfile interface, ColorFormat enum, ByteOrder enum |
| `registry.go` | Global registry, FindProfile(vid, pid), RegisterProfile() |
| `qtkeji.go` | QTKeJi/AIDA64: 480x320, RGB565-BE, SCSI vendor commands |
| `generic.go` | Fallback for unknown devices |
| `generator.go` | Generate skeleton profile from user input |

### DeviceProfile Interface

```go
type ColorFormat int
const (
    RGB565 ColorFormat = iota
    RGB888
)

type ByteOrder int
const (
    BigEndian ByteOrder = iota
    LittleEndian
)

type DeviceProfile interface {
    // Identity
    ID() string                      // "qtkeji", "ax206"
    Name() string                    // "QTKeJi USB Display"
    Description() string             // "480x320 AIDA64-compatible display"
    VendorID() uint16
    ProductID() uint16
    Matches(vid, pid uint16) bool    // Can match multiple VID/PIDs

    // Display properties
    Width() int
    Height() int
    ColorFormat() ColorFormat
    ByteOrder() ByteOrder
    BufferSize() int                 // Width * Height * BytesPerPixel

    // Backlight (0 = not supported)
    MaxBrightness() int

    // Protocol - build raw USB commands
    BlitCommand(x, y, w, h int, dataLen int) []byte
    BacklightCommand(level int) []byte
    ParseResponse([]byte) error

    // Pixel conversion
    ConvertImage(img image.Image) []byte
}
```

### Registry Functions

```go
// pkg/device/registry.go
func Register(profile DeviceProfile)
func FindByVIDPID(vid, pid uint16) DeviceProfile
func FindByID(id string) DeviceProfile
func All() []DeviceProfile
```

### Modify Existing Files

| File | Changes |
|------|---------|
| `pkg/panel/device.go` | Accept DeviceProfile, use profile for dimensions/commands |
| `pkg/panel/protocol.go` | Keep CBW/CSW utilities, move QTKeJi-specific to profile |
| `pkg/config/discovery.go` | Use device.All() for known devices |
| `pkg/config/config.go` | Add ProfileID field to saved config |
| `pkg/renderer/renderer.go` | Accept Width/Height from profile instead of hardcoded |
| `cmd/run.go` | Get dimensions from device profile |
| `cmd/device.go` | Show profile info in device list/info |

---

## Phase 2: Device Creation CLI

### New Command: `sensorpanel device create`

Interactive wizard that generates a skeleton device profile.

**Prompts:**
1. Device ID (slug, e.g., "my-device")
2. Device Name (human-readable)
3. Description
4. Vendor ID (hex, e.g., 0x1908)
5. Product ID (hex)
6. Display Width (pixels)
7. Display Height (pixels)
8. Color Format (RGB565 / RGB888)
9. Byte Order (big-endian / little-endian)
10. Max Brightness (0 = no backlight control)
11. Protocol base (SCSI vendor / raw bulk)

**Output:**
- Creates `pkg/device/<id>.go` with skeleton implementation
- Prints instructions for completing the protocol methods
- Optionally adds to registry (or prints manual instructions)

### Generator Template

```go
// pkg/device/generator.go
type DeviceSpec struct {
    ID            string
    Name          string
    Description   string
    VendorID      uint16
    ProductID     uint16
    Width         int
    Height        int
    ColorFormat   ColorFormat
    ByteOrder     ByteOrder
    MaxBrightness int
}

func GenerateProfile(spec DeviceSpec) (string, error) {
    // Returns Go source code for the profile
}
```

---

## Phase 3: Theme Development Streamlining

### New Files in `pkg/theme/`

| File | Purpose |
|------|---------|
| `devserver.go` | Orchestrate dev experience (WS + Vite + browser) |
| `packagemanager.go` | Detect npm/yarn/pnpm/bun from lockfiles |
| `browser.go` | Cross-platform browser opening |
| `builder.go` | Run theme build, check if dist outdated |

### Package Manager Detection

```go
// Priority order:
// 1. bun.lockb → bun
// 2. pnpm-lock.yaml → pnpm
// 3. yarn.lock → yarn
// 4. package-lock.json → npm
// 5. package.json "packageManager" field
// 6. fallback: npm

type PackageManager string
const (
    NPM  PackageManager = "npm"
    Yarn PackageManager = "yarn"
    PNPM PackageManager = "pnpm"
    Bun  PackageManager = "bun"
)

func Detect(themeDir string) PackageManager
func (pm PackageManager) InstallCmd() []string
func (pm PackageManager) DevCmd() []string
func (pm PackageManager) BuildCmd() []string
```

### Dev Server Orchestration

```go
// pkg/theme/devserver.go
type DevServer struct {
    ThemeDir string
    WSPort   int      // default 19847, auto-increment if busy
    VitePort int      // default 15173 (avoid conflict with 5173)
}

func (d *DevServer) Start(ctx context.Context) error {
    // 1. Detect package manager
    // 2. Run install if node_modules missing
    // 3. Start WebSocket sensor server (port 19847+)
    // 4. Spawn Vite dev server (port 15173)
    // 5. Wait for Vite ready
    // 6. Open browser
    // 7. Stream combined logs with prefixes
}
```

**Log output format:**
```
[ws]    Sensor server listening on port 19847
[vite]  VITE v5.4.0  ready in 250 ms
[vite]  ➜  Local:   http://localhost:15173/
[ws]    Client connected from 127.0.0.1
[vite]  hmr update /src/App.tsx
```

### Build & Outdated Check

```go
// pkg/theme/builder.go
func Build(themeDir string) error {
    // 1. Detect package manager
    // 2. Run build command
    // 3. Verify dist/index.html exists
}

func IsOutdated(themeDir string) bool {
    // Compare newest mtime in src/ vs dist/index.html mtime
}
```

### CLI Updates

| Command | Behavior |
|---------|----------|
| `theme dev [name]` | If no name, use selected theme. Start dev server. |
| `theme build [name]` | If no name, use selected theme. Run build. |
| `theme create <name>` | Create React+TS template with SDK |

### Run Command Warning

```go
// cmd/run.go
if selectedTheme != "" {
    if theme.IsOutdated(themeDir) {
        fmt.Fprintf(os.Stderr,
            "Warning: Theme '%s' source is newer than build. "+
            "Run 'sensorpanel theme build' to rebuild.\n",
            selectedTheme)
    }
}
```

---

## Phase 4: React + TypeScript Template with SDK

### Template Structure

```
pkg/theme/template/
├── package.json
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts
├── eslint.config.js
├── index.html
├── src/
│   ├── main.tsx
│   ├── App.tsx
│   ├── App.css
│   └── vite-env.d.ts
└── lib/
    └── sensorpanel/
        ├── index.ts
        ├── client.ts
        ├── types.ts
        └── hooks.ts
```

### SDK: `lib/sensorpanel/types.ts`

```typescript
export interface SensorData {
  cpu: CpuData;
  gpu: GpuData;
  ram: RamData;
  disk: Record<string, DiskData>;
  network: Record<string, NetworkData>;
  timestamp: number;
}

export interface CpuData {
  temperature: number;   // Celsius
  load: number;          // 0-100
  frequency: number;     // MHz
  cores?: number;
}

export interface GpuData {
  name?: string;
  temperature: number;
  load: number;          // 0-100
  memoryUsed: number;    // MB
  memoryTotal: number;   // MB
  power?: number;        // Watts
}

export interface RamData {
  used: number;          // MB
  total: number;         // MB
  percent: number;       // 0-100
}

export interface DiskData {
  mountpoint: string;
  used: number;          // GB
  total: number;         // GB
  percent: number;       // 0-100
}

export interface NetworkData {
  interface: string;
  rxRate: number;        // bytes/sec
  txRate: number;        // bytes/sec
  rxTotal?: number;      // total bytes
  txTotal?: number;
}

export type ConnectionStatus = "connecting" | "connected" | "disconnected" | "error";
```

### SDK: `lib/sensorpanel/client.ts`

```typescript
import type { SensorData, ConnectionStatus } from "./types";

const DEFAULT_PORTS = [19847, 19848, 19849, 19850, 19851];
const RECONNECT_DELAY = 2000;

type DataListener = (data: SensorData) => void;
type StatusListener = (status: ConnectionStatus) => void;

class SensorPanelClient {
  private ws: WebSocket | null = null;
  private dataListeners = new Set<DataListener>();
  private statusListeners = new Set<StatusListener>();
  private status: ConnectionStatus = "disconnected";
  private reconnectTimer: number | null = null;

  async connect(): Promise<void> {
    this.setStatus("connecting");

    for (const port of DEFAULT_PORTS) {
      try {
        await this.tryConnect(port);
        this.setStatus("connected");
        return;
      } catch {
        continue;
      }
    }

    this.setStatus("error");
    this.scheduleReconnect();
  }

  private tryConnect(port: number): Promise<void> {
    return new Promise((resolve, reject) => {
      const ws = new WebSocket(`ws://localhost:${port}`);
      const timeout = setTimeout(() => {
        ws.close();
        reject(new Error("Connection timeout"));
      }, 1000);

      ws.onopen = () => {
        clearTimeout(timeout);
        this.ws = ws;
        this.setupListeners();
        resolve();
      };

      ws.onerror = () => {
        clearTimeout(timeout);
        reject(new Error("Connection failed"));
      };
    });
  }

  private setupListeners(): void {
    if (!this.ws) return;

    this.ws.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data) as SensorData;
        this.dataListeners.forEach((fn) => fn(data));
      } catch {
        console.error("Failed to parse sensor data");
      }
    };

    this.ws.onclose = () => {
      this.setStatus("disconnected");
      this.scheduleReconnect();
    };

    this.ws.onerror = () => {
      this.setStatus("error");
    };
  }

  private setStatus(status: ConnectionStatus): void {
    this.status = status;
    this.statusListeners.forEach((fn) => fn(status));
  }

  private scheduleReconnect(): void {
    if (this.reconnectTimer) return;
    this.reconnectTimer = window.setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, RECONNECT_DELAY);
  }

  subscribe(callback: DataListener): () => void {
    this.dataListeners.add(callback);
    return () => this.dataListeners.delete(callback);
  }

  onStatusChange(callback: StatusListener): () => void {
    this.statusListeners.add(callback);
    callback(this.status);
    return () => this.statusListeners.delete(callback);
  }

  disconnect(): void {
    if (this.reconnectTimer) {
      clearTimeout(this.reconnectTimer);
      this.reconnectTimer = null;
    }
    this.ws?.close();
    this.ws = null;
    this.setStatus("disconnected");
  }

  getStatus(): ConnectionStatus {
    return this.status;
  }
}

export const client = new SensorPanelClient();
export { SensorPanelClient };
```

### SDK: `lib/sensorpanel/hooks.ts`

```typescript
import { useState, useEffect } from "react";
import { client } from "./client";
import type { SensorData, ConnectionStatus, CpuData, GpuData, RamData } from "./types";

export function useSensorData(): SensorData | null {
  const [data, setData] = useState<SensorData | null>(null);

  useEffect(() => {
    void client.connect();
    const unsubscribe = client.subscribe(setData);
    return () => {
      unsubscribe();
    };
  }, []);

  return data;
}

export function useConnectionStatus(): ConnectionStatus {
  const [status, setStatus] = useState<ConnectionStatus>(client.getStatus());

  useEffect(() => {
    return client.onStatusChange(setStatus);
  }, []);

  return status;
}

export function useCpuData(): CpuData | null {
  const data = useSensorData();
  return data?.cpu ?? null;
}

export function useGpuData(): GpuData | null {
  const data = useSensorData();
  return data?.gpu ?? null;
}

export function useRamData(): RamData | null {
  const data = useSensorData();
  return data?.ram ?? null;
}

export function formatBytes(bytes: number): string {
  if (bytes >= 1e9) return `${(bytes / 1e9).toFixed(1)} GB`;
  if (bytes >= 1e6) return `${(bytes / 1e6).toFixed(1)} MB`;
  if (bytes >= 1e3) return `${(bytes / 1e3).toFixed(1)} KB`;
  return `${bytes} B`;
}

export function formatRate(bytesPerSec: number): string {
  if (bytesPerSec >= 1e6) return `${(bytesPerSec / 1e6).toFixed(1)} MB/s`;
  if (bytesPerSec >= 1e3) return `${(bytesPerSec / 1e3).toFixed(0)} KB/s`;
  return `${bytesPerSec} B/s`;
}
```

### SDK: `lib/sensorpanel/index.ts`

```typescript
// Client
export { client, SensorPanelClient } from "./client";

// Hooks
export {
  useSensorData,
  useConnectionStatus,
  useCpuData,
  useGpuData,
  useRamData,
  formatBytes,
  formatRate,
} from "./hooks";

// Types
export type {
  SensorData,
  CpuData,
  GpuData,
  RamData,
  DiskData,
  NetworkData,
  ConnectionStatus,
} from "./types";
```

### Starter Template: `src/App.tsx`

```typescript
import { useSensorData, useConnectionStatus, formatRate } from "../lib/sensorpanel";
import "./App.css";

function App() {
  const data = useSensorData();
  const status = useConnectionStatus();

  if (status === "connecting") {
    return <div className="status">Connecting to SensorPanel...</div>;
  }

  if (status === "error" || status === "disconnected") {
    return (
      <div className="status error">
        Disconnected. Retrying...
        <br />
        <small>Make sure 'sensorpanel theme dev' is running</small>
      </div>
    );
  }

  if (!data) {
    return <div className="status">Waiting for data...</div>;
  }

  return (
    <div className="dashboard">
      <div className="metric cpu">
        <div className="label">CPU</div>
        <div className="value">{data.cpu.load.toFixed(0)}%</div>
        <div className="sub">{data.cpu.temperature.toFixed(0)}°C</div>
      </div>

      <div className="metric gpu">
        <div className="label">GPU</div>
        <div className="value">{data.gpu.load.toFixed(0)}%</div>
        <div className="sub">{data.gpu.temperature.toFixed(0)}°C</div>
      </div>

      <div className="metric ram">
        <div className="label">RAM</div>
        <div className="value">{data.ram.percent.toFixed(0)}%</div>
        <div className="sub">
          {(data.ram.used / 1024).toFixed(1)} / {(data.ram.total / 1024).toFixed(1)} GB
        </div>
      </div>

      {Object.entries(data.network).map(([iface, net]) => (
        <div className="metric network" key={iface}>
          <div className="label">{iface}</div>
          <div className="sub">
            ↓ {formatRate(net.rxRate)} ↑ {formatRate(net.txRate)}
          </div>
        </div>
      ))}
    </div>
  );
}

export default App;
```

### Template `src/App.css`

```css
:root {
  --bg-color: #1a1a2e;
  --card-bg: #16213e;
  --text-primary: #eee;
  --text-secondary: #888;
  --accent-cpu: #e94560;
  --accent-gpu: #0f3460;
  --accent-ram: #533483;
  --accent-network: #1a508b;
}

* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: var(--bg-color);
  color: var(--text-primary);
  width: 480px;
  height: 320px;
  overflow: hidden;
}

.status {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 18px;
}

.status.error {
  color: var(--accent-cpu);
}

.status small {
  margin-top: 8px;
  color: var(--text-secondary);
  font-size: 12px;
}

.dashboard {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  grid-template-rows: repeat(2, 1fr);
  gap: 8px;
  padding: 8px;
  height: 100%;
}

.metric {
  background: var(--card-bg);
  border-radius: 8px;
  padding: 12px;
  display: flex;
  flex-direction: column;
  justify-content: center;
}

.metric .label {
  font-size: 14px;
  color: var(--text-secondary);
  text-transform: uppercase;
  letter-spacing: 1px;
}

.metric .value {
  font-size: 36px;
  font-weight: bold;
  margin: 4px 0;
}

.metric .sub {
  font-size: 12px;
  color: var(--text-secondary);
}

.metric.cpu { border-left: 4px solid var(--accent-cpu); }
.metric.gpu { border-left: 4px solid var(--accent-gpu); }
.metric.ram { border-left: 4px solid var(--accent-ram); }
.metric.network { border-left: 4px solid var(--accent-network); }
```

### Template `vite.config.ts`

```typescript
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 15173,
    strictPort: false,
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
  },
});
```

### Template `eslint.config.js`

```javascript
import js from "@eslint/js";
import tseslint from "typescript-eslint";
import reactHooks from "eslint-plugin-react-hooks";
import reactRefresh from "eslint-plugin-react-refresh";

export default tseslint.config(
  { ignores: ["dist", "node_modules"] },
  {
    extends: [
      js.configs.recommended,
      ...tseslint.configs.strictTypeChecked,
    ],
    files: ["**/*.{ts,tsx}"],
    languageOptions: {
      parserOptions: {
        project: ["./tsconfig.json", "./tsconfig.node.json"],
      },
    },
    plugins: {
      "react-hooks": reactHooks,
      "react-refresh": reactRefresh,
    },
    rules: {
      ...reactHooks.configs.recommended.rules,
      "react-refresh/only-export-components": [
        "warn",
        { allowConstantExport: true },
      ],
    },
  }
);
```

### Template `package.json`

```json
{
  "name": "{{THEME_NAME}}",
  "private": true,
  "version": "0.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite",
    "build": "tsc -b && vite build",
    "lint": "eslint .",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.3.1",
    "react-dom": "^18.3.1"
  },
  "devDependencies": {
    "@eslint/js": "^9.17.0",
    "@types/react": "^18.3.18",
    "@types/react-dom": "^18.3.5",
    "@vitejs/plugin-react": "^4.3.4",
    "eslint": "^9.17.0",
    "eslint-plugin-react-hooks": "^5.1.0",
    "eslint-plugin-react-refresh": "^0.4.16",
    "typescript": "~5.6.2",
    "typescript-eslint": "^8.18.2",
    "vite": "^6.0.5"
  }
}
```

### Template `tsconfig.json`

```json
{
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.app.tsbuildinfo",
    "target": "ES2020",
    "useDefineForClassFields": true,
    "lib": ["ES2020", "DOM", "DOM.Iterable"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "jsx": "react-jsx",
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true,
    "noUncheckedIndexedAccess": true
  },
  "include": ["src", "lib"]
}
```

### Template `tsconfig.node.json`

```json
{
  "compilerOptions": {
    "tsBuildInfoFile": "./node_modules/.tmp/tsconfig.node.tsbuildinfo",
    "target": "ES2022",
    "lib": ["ES2023"],
    "module": "ESNext",
    "skipLibCheck": true,
    "moduleResolution": "bundler",
    "allowImportingTsExtensions": true,
    "isolatedModules": true,
    "moduleDetection": "force",
    "noEmit": true,
    "strict": true,
    "noUnusedLocals": true,
    "noUnusedParameters": true,
    "noFallthroughCasesInSwitch": true
  },
  "include": ["vite.config.ts"]
}
```

### Template `index.html`

```html
<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=480, height=320, initial-scale=1.0" />
    <title>SensorPanel Theme</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
```

### Template `src/main.tsx`

```typescript
import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Root element not found");

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>
);
```

### Template `src/vite-env.d.ts`

```typescript
/// <reference types="vite/client" />
```

---

## Phase 5: Mage Build System

### New Directory: `magefiles/`

```
magefiles/
└── magefile.go
```

### `magefiles/magefile.go`

```go
//go:build mage

package main

import (
    "fmt"
    "os"
    "runtime"

    "github.com/magefile/mage/mg"
    "github.com/magefile/mage/sh"
)

var Default = Build

// Build compiles sensorpanel for the current platform
func Build() error {
    fmt.Println("Building sensorpanel...")
    return sh.Run("go", "build", "-o", binaryName(), ".")
}

// Test runs all tests
func Test() error {
    fmt.Println("Running tests...")
    return sh.Run("go", "test", "-v", "./...")
}

// Lint runs golangci-lint
func Lint() error {
    fmt.Println("Running linter...")
    return sh.Run("golangci-lint", "run", "./...")
}

// Install builds and installs to GOPATH/bin
func Install() error {
    fmt.Println("Installing sensorpanel...")
    return sh.Run("go", "install", ".")
}

// Clean removes build artifacts
func Clean() error {
    fmt.Println("Cleaning...")
    os.Remove("sensorpanel")
    os.Remove("sensorpanel.exe")
    return os.RemoveAll("dist")
}

// Release cross-compiles for all platforms
func Release() error {
    mg.Deps(Clean)

    if err := os.MkdirAll("dist", 0755); err != nil {
        return err
    }

    targets := []struct {
        goos   string
        goarch string
    }{
        {"linux", "amd64"},
        {"linux", "arm64"},
        {"darwin", "amd64"},
        {"darwin", "arm64"},
        {"windows", "amd64"},
    }

    for _, t := range targets {
        fmt.Printf("Building for %s/%s...\n", t.goos, t.goarch)

        ext := ""
        if t.goos == "windows" {
            ext = ".exe"
        }

        output := fmt.Sprintf("dist/sensorpanel-%s-%s%s", t.goos, t.goarch, ext)

        env := map[string]string{
            "GOOS":        t.goos,
            "GOARCH":      t.goarch,
            "CGO_ENABLED": "0",
        }

        if err := sh.RunWith(env, "go", "build", "-o", output, "-ldflags", "-s -w", "."); err != nil {
            return err
        }
    }

    return nil
}

// Dev builds and runs in development mode
func Dev() error {
    mg.Deps(Build)
    return sh.Run("./"+binaryName(), "run")
}

func binaryName() string {
    if runtime.GOOS == "windows" {
        return "sensorpanel.exe"
    }
    return "sensorpanel"
}
```

### `go.mod` Addition

```
require github.com/magefile/mage v1.15.0
```

---

## Phase 6: Open Source Preparation

### `CONTRIBUTING.md`

```markdown
# Contributing to SensorPanel

Thank you for your interest in contributing!

## Development Setup

1. Prerequisites:
   - Go 1.21+
   - Node.js 18+ (for theme development)
   - libusb (Linux: `libusb-1.0-0-dev`, macOS: `brew install libusb`)

2. Clone and build:
   ```bash
   git clone https://github.com/yourusername/sensorpanel.git
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

4. Share your theme by submitting it to the community themes repo.

## Code Style

- Run `go fmt` before committing
- Run `golangci-lint run` to check for issues
- Follow existing code patterns
- Add tests for new functionality

## Pull Request Guidelines

1. Create a feature branch from `main`
2. Make your changes with clear commit messages
3. Run tests: `go test ./...`
4. Run linter: `golangci-lint run`
5. Submit PR with clear description of changes

## Reporting Issues

- Use the appropriate issue template
- Include device info (VID/PID, lsusb output) for hardware issues
- Include logs and error messages
- Include steps to reproduce
```

### `docs/adding-devices.md`

```markdown
# Adding Support for New Devices

This guide explains how to add support for a new USB display device.

## Quick Start

Run the device creation wizard:

```bash
./sensorpanel device create
```

Follow the prompts to generate a skeleton device profile.

## Understanding Device Profiles

A device profile implements the `DeviceProfile` interface defined in `pkg/device/profile.go`.

Key methods you need to implement:

| Method | Purpose |
|--------|---------|
| `ID()` | Unique identifier (e.g., "qtkeji") |
| `Matches(vid, pid)` | Return true if this profile supports the device |
| `Width()`, `Height()` | Display resolution |
| `ColorFormat()` | RGB565 or RGB888 |
| `ByteOrder()` | BigEndian or LittleEndian |
| `BlitCommand()` | Build command bytes to send image data |
| `BacklightCommand()` | Build command bytes for backlight control |
| `ConvertImage()` | Convert Go image to device pixel format |

## Protocol Research

Before implementing, you need to understand your device's USB protocol:

### 1. Identify Your Device

```bash
lsusb
# Find your device, note VID:PID
lsusb -d XXXX:YYYY -v
```

### 2. Capture USB Traffic

Use Wireshark with USBPcap (Windows) or usbmon (Linux):

1. Install Wireshark with USB capture support
2. Run the manufacturer's software that drives the display
3. Capture the USB traffic
4. Analyze the packets

### 3. Key Things to Look For

- **Command structure**: Most devices use SCSI CBW or custom bulk transfers
- **Pixel format**: RGB565 (16-bit) or RGB888 (24-bit)
- **Byte order**: Big-endian or little-endian
- **Image transfer**: How coordinates and dimensions are encoded
- **Backlight control**: Command bytes for brightness levels

## Example Implementation

See `pkg/device/qtkeji.go` for a complete example:

```go
type QTKeJiProfile struct{}

func (p *QTKeJiProfile) ID() string { return "qtkeji" }

func (p *QTKeJiProfile) Matches(vid, pid uint16) bool {
    return vid == 0x1908 && (pid == 0x0102 || pid == 0x0103)
}

func (p *QTKeJiProfile) BlitCommand(x, y, w, h int, dataLen int) []byte {
    // Build SCSI CBW with vendor command
    cmd := make([]byte, 16)
    cmd[0] = 0xCD  // Vendor prefix
    // ... fill in command bytes
    return BuildCBW(cmd, dataLen, DirOut)
}
```

## Testing

1. Build: `go build .`
2. List devices: `./sensorpanel device list`
3. Select your device: `./sensorpanel device select`
4. Test pattern: `./sensorpanel panel test`
5. Run dashboard: `./sensorpanel run`

## Submitting Your Profile

1. Fork the repository
2. Add your profile in `pkg/device/`
3. Register it in `pkg/device/registry.go`
4. Add yourself to CONTRIBUTORS.md
5. Submit a pull request with:
   - Device name and manufacturer
   - Link to purchase (if available)
   - Brief description of protocol
```

### `docs/creating-themes.md`

```markdown
# Creating Themes

Themes are React + TypeScript applications that display sensor data on the USB panel.

## Quick Start

```bash
# Create a new theme
./sensorpanel theme create my-theme

# Start development (opens browser with hot reload)
./sensorpanel theme dev my-theme

# Build for production
./sensorpanel theme build my-theme

# Use with your panel
./sensorpanel theme select my-theme
./sensorpanel run
```

## Theme Structure

```
my-theme/
├── package.json
├── tsconfig.json
├── vite.config.ts
├── index.html
├── src/
│   ├── main.tsx      # Entry point
│   ├── App.tsx       # Main component
│   └── App.css       # Styles
└── lib/
    └── sensorpanel/  # SDK (auto-generated)
        ├── index.ts
        ├── client.ts
        ├── hooks.ts
        └── types.ts
```

## Using the SDK

The SDK provides React hooks for accessing sensor data:

```tsx
import { useSensorData, useConnectionStatus } from "../lib/sensorpanel";

function App() {
  const data = useSensorData();
  const status = useConnectionStatus();

  if (status !== "connected" || !data) {
    return <div>Connecting...</div>;
  }

  return (
    <div>
      <p>CPU: {data.cpu.load}%</p>
      <p>GPU: {data.gpu.temperature}°C</p>
    </div>
  );
}
```

### Available Hooks

| Hook | Returns | Description |
|------|---------|-------------|
| `useSensorData()` | `SensorData \| null` | All sensor data |
| `useConnectionStatus()` | `ConnectionStatus` | "connecting", "connected", "disconnected", "error" |
| `useCpuData()` | `CpuData \| null` | CPU data only |
| `useGpuData()` | `GpuData \| null` | GPU data only |
| `useRamData()` | `RamData \| null` | RAM data only |

### Utility Functions

```tsx
import { formatBytes, formatRate } from "../lib/sensorpanel";

formatBytes(1073741824);  // "1.0 GB"
formatRate(1048576);      // "1.0 MB/s"
```

## Data Types

```typescript
interface SensorData {
  cpu: {
    temperature: number;  // Celsius
    load: number;         // 0-100
    frequency: number;    // MHz
  };
  gpu: {
    temperature: number;
    load: number;
    memoryUsed: number;   // MB
    memoryTotal: number;  // MB
    power?: number;       // Watts
  };
  ram: {
    used: number;         // MB
    total: number;        // MB
    percent: number;
  };
  disk: Record<string, {
    used: number;         // GB
    total: number;        // GB
    percent: number;
  }>;
  network: Record<string, {
    rxRate: number;       // bytes/sec
    txRate: number;       // bytes/sec
  }>;
}
```

## Display Dimensions

The default display is 480x320 pixels. Design your theme for this resolution:

```css
body {
  width: 480px;
  height: 320px;
  overflow: hidden;
}
```

## Tips

1. **Keep it simple**: The display is small, focus on key metrics
2. **High contrast**: Use light text on dark background for readability
3. **Large fonts**: Aim for 24px+ for values you need to read at a glance
4. **Test on device**: Colors may look different on the LCD
5. **Avoid animations**: They may not render smoothly
```

### `.github/ISSUE_TEMPLATE/bug_report.md`

```markdown
---
name: Bug Report
about: Report a bug in sensorpanel
title: '[Bug] '
labels: 'bug'
---

## Description

A clear description of the bug.

## Steps to Reproduce

1. Run '...'
2. Click on '...'
3. See error

## Expected Behavior

What you expected to happen.

## Actual Behavior

What actually happened.

## Environment

- OS: [e.g., Ubuntu 24.04, Windows 11, macOS 14]
- Go version: [e.g., 1.22.0]
- sensorpanel version: [e.g., 1.0.0 or commit hash]

## Device Info (if applicable)

```
Paste output of: lsusb -d XXXX:YYYY -v
```

## Logs

```
Paste any relevant error messages or logs
```

## Additional Context

Any other information that might help.
```

### `.github/ISSUE_TEMPLATE/feature_request.md`

```markdown
---
name: Feature Request
about: Suggest a new feature
title: '[Feature] '
labels: 'enhancement'
---

## Description

A clear description of the feature you'd like.

## Use Case

Why do you need this feature? What problem does it solve?

## Proposed Solution

How do you think this should work?

## Alternatives Considered

Any alternative solutions you've thought about.

## Additional Context

Any other information, mockups, or examples.
```

### `.github/ISSUE_TEMPLATE/new_device.md`

```markdown
---
name: New Device Support
about: Request or contribute support for a new USB display
title: '[Device] '
labels: 'new-device'
---

## Device Information

**Manufacturer/Brand:**

**Model/Product Name:**

**Vendor ID (hex):** 0x

**Product ID (hex):** 0x

**Display Resolution:** x

**Purchase Link (if available):**

## USB Descriptor

```
Paste output of: lsusb -d XXXX:YYYY -v
```

## Protocol Information (if known)

- [ ] I have USB traffic captures
- [ ] I've identified the pixel format (RGB565/RGB888)
- [ ] I've identified the byte order (big/little endian)
- [ ] I've identified the command structure

## Contribution

- [ ] I'm willing to help test
- [ ] I'm willing to implement the device profile
- [ ] I can provide USB captures

## Additional Notes

Any other information about the device, similar devices, or existing software that supports it.
```

### `.github/workflows/ci.yml`

```yaml
name: CI

on:
  push:
    branches: [main, master]
  pull_request:
    branches: [main, master]

jobs:
  build:
    runs-on: ${{ matrix.os }}
    strategy:
      fail-fast: false
      matrix:
        os: [ubuntu-latest, macos-latest, windows-latest]
        go: ['1.21', '1.22']

    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: ${{ matrix.go }}

      - name: Install libusb (Ubuntu)
        if: matrix.os == 'ubuntu-latest'
        run: sudo apt-get update && sudo apt-get install -y libusb-1.0-0-dev

      - name: Install libusb (macOS)
        if: matrix.os == 'macos-latest'
        run: brew install libusb pkg-config

      - name: Build
        run: go build -v ./...

      - name: Test
        run: go test -v ./...

  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.22'

      - name: Install libusb
        run: sudo apt-get update && sudo apt-get install -y libusb-1.0-0-dev

      - name: golangci-lint
        uses: golangci/golangci-lint-action@v4
        with:
          version: latest
```

---

## Implementation Checklist

### Phase 1: Device Profile System
- [ ] Create `pkg/device/profile.go` - interface and types
- [ ] Create `pkg/device/registry.go` - registration and lookup
- [ ] Create `pkg/device/qtkeji.go` - extract current device logic
- [ ] Create `pkg/device/generic.go` - fallback profile
- [ ] Refactor `pkg/panel/device.go` - use profiles
- [ ] Refactor `pkg/panel/protocol.go` - shared utilities only
- [ ] Update `pkg/config/discovery.go` - use registry
- [ ] Update `pkg/config/config.go` - add ProfileID field
- [ ] Update `pkg/renderer/renderer.go` - dynamic dimensions
- [ ] Update `cmd/run.go` - get dimensions from profile
- [ ] Update `cmd/device.go` - show profile info

### Phase 2: Device Creation CLI
- [ ] Create `pkg/device/generator.go` - profile code generator
- [ ] Add `device create` command to `cmd/device.go`
- [ ] Add interactive prompts
- [ ] Generate skeleton Go file
- [ ] Print next steps for contributor

### Phase 3: Theme Development
- [ ] Create `pkg/theme/packagemanager.go`
- [ ] Create `pkg/theme/browser.go`
- [ ] Create `pkg/theme/devserver.go`
- [ ] Create `pkg/theme/builder.go`
- [ ] Update `cmd/theme.go` - dev, build commands
- [ ] Update `cmd/run.go` - outdated warning

### Phase 4: React+TypeScript Template
- [ ] Create template directory structure
- [ ] Create `lib/sensorpanel/types.ts`
- [ ] Create `lib/sensorpanel/client.ts`
- [ ] Create `lib/sensorpanel/hooks.ts`
- [ ] Create `lib/sensorpanel/index.ts`
- [ ] Create `src/App.tsx` starter
- [ ] Create `src/App.css` starter
- [ ] Create all config files (package.json, tsconfig, vite, eslint)
- [ ] Update `pkg/theme/template.go` to embed new template

### Phase 5: Mage Build System
- [ ] Create `magefiles/magefile.go`
- [ ] Add mage to `go.mod`
- [ ] Test all targets (build, test, lint, release)

### Phase 6: Open Source Prep
- [ ] Create `CONTRIBUTING.md`
- [ ] Create `docs/adding-devices.md`
- [ ] Create `docs/creating-themes.md`
- [ ] Create `.github/ISSUE_TEMPLATE/bug_report.md`
- [ ] Create `.github/ISSUE_TEMPLATE/feature_request.md`
- [ ] Create `.github/ISSUE_TEMPLATE/new_device.md`
- [ ] Create `.github/workflows/ci.yml`
- [ ] Update `README.md` - add contributor section, badges

---

## File Summary

### New Files (~35)

```
pkg/device/
├── profile.go
├── registry.go
├── qtkeji.go
├── generic.go
└── generator.go

pkg/theme/
├── packagemanager.go
├── browser.go
├── devserver.go
├── builder.go
└── template/
    ├── package.json
    ├── tsconfig.json
    ├── tsconfig.node.json
    ├── vite.config.ts
    ├── eslint.config.js
    ├── index.html
    ├── .gitignore
    ├── src/
    │   ├── main.tsx
    │   ├── App.tsx
    │   ├── App.css
    │   └── vite-env.d.ts
    └── lib/
        └── sensorpanel/
            ├── index.ts
            ├── client.ts
            ├── types.ts
            └── hooks.ts

magefiles/
└── magefile.go

docs/
├── adding-devices.md
├── creating-themes.md
└── protocol.md

.github/
├── ISSUE_TEMPLATE/
│   ├── bug_report.md
│   ├── feature_request.md
│   └── new_device.md
└── workflows/
    └── ci.yml

CONTRIBUTING.md
```

### Modified Files (~10)

```
pkg/panel/device.go
pkg/panel/protocol.go
pkg/config/config.go
pkg/config/discovery.go
pkg/renderer/renderer.go
pkg/theme/template.go
cmd/run.go
cmd/device.go
cmd/theme.go
go.mod
README.md
```
