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
├── eslint.config.js
├── index.html
├── src/
│   ├── main.tsx      # Entry point
│   ├── App.tsx       # Main component
│   ├── App.css       # Styles
│   └── vite-env.d.ts
├── lib/
│   └── sensorpanel/  # SDK (auto-generated)
│       ├── index.ts
│       ├── client.ts
│       ├── hooks.ts
│       └── types.ts
└── dist/             # Built output
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
| `useMemoryData()` | `MemoryData \| null` | Memory data only |

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
    temperature?: number;  // Celsius
    load: number;          // 0-100
    frequency?: number;    // MHz
    cores?: number;
  };
  gpu: {
    available: boolean;
    name?: string;
    temperature?: number;
    load?: number;
    memoryUsed?: number;   // MB
    memoryTotal?: number;  // MB
    power?: number;        // Watts
  };
  memory: {
    used: number;          // MB
    total: number;         // MB
    available: number;     // MB
    percent: number;       // 0-100
  };
  disk: Record<string, {
    mountpoint: string;
    used: number;          // GB
    total: number;         // GB
    free: number;          // GB
    percent: number;       // 0-100
  }>;
  network: Record<string, {
    interface: string;
    rxRate: number;        // bytes/sec
    txRate: number;        // bytes/sec
  }>;
}

type ConnectionStatus = "connecting" | "connected" | "disconnected" | "error";
```

## Display Dimensions

The default display is 480x320 pixels. Design your theme for this resolution:

```css
html, body, #root {
  width: 480px;
  height: 320px;
  overflow: hidden;
}
```

## Development Workflow

1. **Create theme**: `sensorpanel theme create my-theme`

2. **Start dev server**: `sensorpanel theme dev my-theme`
   - This starts the WebSocket sensor server
   - Launches Vite dev server with HMR
   - Opens your browser automatically

3. **Edit files**: Changes to `src/` hot-reload instantly

4. **Build**: `sensorpanel theme build my-theme`
   - Runs TypeScript compiler
   - Bundles with Vite
   - Outputs to `dist/`

5. **Use**: `sensorpanel theme select my-theme && sensorpanel run`

## Package Manager Support

SensorPanel auto-detects your preferred package manager:

| Lockfile | Package Manager |
|----------|-----------------|
| `bun.lockb` | bun |
| `pnpm-lock.yaml` | pnpm |
| `yarn.lock` | yarn |
| `package-lock.json` | npm |

You can use any of these - just install dependencies normally:

```bash
npm install     # or yarn, pnpm, bun
```

## Tips

1. **Keep it simple**: The display is small, focus on key metrics
2. **High contrast**: Use light text on dark background for readability
3. **Large fonts**: Aim for 24px+ for values you need to read at a glance
4. **Test on device**: Colors may look different on the LCD
5. **Keep unchanged areas stable**: Regional updates are fastest when only small rectangles change
6. **Avoid full-screen animations**: They defeat regional updates and may not render smoothly on USB panels
7. **Render nothing until data arrives**: Avoid flashing temporary connection text on the physical display
8. **Handle null values**: Sensors may not always return data

## Example Themes

### Minimal

```tsx
function App() {
  const data = useSensorData();
  if (!data) return null;
  
  return (
    <div style={{ fontSize: 48, textAlign: 'center', paddingTop: 120 }}>
      CPU: {data.cpu.load.toFixed(0)}%
    </div>
  );
}
```

### Grid Layout

```tsx
function App() {
  const data = useSensorData();
  if (!data) return null;
  
  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: 8, padding: 8, height: '100%' }}>
      <Metric label="CPU" value={`${data.cpu.load.toFixed(0)}%`} />
      <Metric label="GPU" value={`${data.gpu.load?.toFixed(0) ?? '--'}%`} />
      <Metric label="RAM" value={`${data.memory.percent.toFixed(0)}%`} />
      <Metric label="TEMP" value={`${data.cpu.temperature?.toFixed(0) ?? '--'}°C`} />
    </div>
  );
}
```

## Troubleshooting

### "Disconnected. Retrying..."

Make sure `sensorpanel theme dev` is running. The theme connects to the WebSocket server on port 19847.

### Theme not updating on device

Run `sensorpanel theme build` to rebuild, then restart `sensorpanel run`.

### TypeScript errors

Run `npm run lint` to check for issues. The template uses strict TypeScript settings.
