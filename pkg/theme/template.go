// Package theme - template files for new themes.
package theme

import "fmt"

// getTemplateFiles returns the template files for a new theme.
func getTemplateFiles(themeName string) map[string]string {
	return map[string]string{
		// Config files
		"package.json":       packageJSON(themeName),
		"tsconfig.json":      tsconfigJSON(),
		"tsconfig.node.json": tsconfigNodeJSON(),
		"vite.config.ts":     viteConfigTS(),
		"eslint.config.js":   eslintConfig(),
		"index.html":         indexHTML(themeName),
		".gitignore":         gitignore(),

		// Source files
		"src/main.tsx":      mainTSX(),
		"src/App.tsx":       appTSX(),
		"src/App.css":       appCSS(),
		"src/vite-env.d.ts": viteEnvDTS(),

		// SDK
		"lib/sensorpanel/index.ts":  sdkIndexTS(),
		"lib/sensorpanel/types.ts":  sdkTypesTS(),
		"lib/sensorpanel/client.ts": sdkClientTS(),
		"lib/sensorpanel/hooks.ts":  sdkHooksTS(),

		// Pre-built dist (works immediately)
		"dist/index.html": distIndexHTML(themeName),
	}
}

func packageJSON(name string) string {
	return fmt.Sprintf(`{
  "name": "%s",
  "private": true,
  "version": "1.0.0",
  "description": "A sensorpanel theme",
  "type": "module",
  "width": 480,
  "height": 320,
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
`, name)
}

func tsconfigJSON() string {
	return `{
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
`
}

func tsconfigNodeJSON() string {
	return `{
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
  "include": ["vite.config.ts", "eslint.config.js"]
}
`
}

func viteConfigTS() string {
	return `import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: "./",
  server: {
    port: 15173,
    strictPort: false,
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    assetsDir: "assets",
    assetsInlineLimit: 100000,
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        manualChunks: undefined,
        inlineDynamicImports: true,
      },
    },
  },
});
`
}

func eslintConfig() string {
	return `import js from "@eslint/js";
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
`
}

func indexHTML(name string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=480, height=320, initial-scale=1.0" />
    <title>%s - SensorPanel</title>
    <style>
      * { margin: 0; padding: 0; box-sizing: border-box; }
      html, body, #root { width: 480px; height: 320px; overflow: hidden; }
    </style>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
`, name)
}

func gitignore() string {
	return `node_modules/
dist/
*.log
.DS_Store
`
}

func mainTSX() string {
	return `import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import App from "./App";
import "./App.css";

const rootElement = document.getElementById("root");
if (!rootElement) throw new Error("Root element not found");

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>
);
`
}

func appTSX() string {
	return `import { useSensorData, formatRate } from "../lib/sensorpanel";
import "./App.css";

function App() {
  const data = useSensorData();

  if (!data) {
    return <div className="status" aria-label="Waiting for sensor data" />;
  }

  return (
    <div className="dashboard">
      <div className="metric cpu">
        <div className="label">CPU</div>
        <div className="value">{data.cpu.load.toFixed(0)}%</div>
        <div className="sub">{data.cpu.temperature?.toFixed(0) ?? "--"}°C</div>
      </div>

      <div className="metric gpu">
        <div className="label">GPU</div>
        <div className="value">{data.gpu.load?.toFixed(0) ?? "--"}%</div>
        <div className="sub">{data.gpu.temperature?.toFixed(0) ?? "--"}°C</div>
      </div>

      <div className="metric ram">
        <div className="label">RAM</div>
        <div className="value">{data.memory.percent.toFixed(0)}%</div>
        <div className="sub">
          {(data.memory.used / 1024).toFixed(1)} / {(data.memory.total / 1024).toFixed(1)} GB
        </div>
      </div>

      <div className="metric network">
        <div className="label">NET</div>
        {Object.entries(data.network).length > 0 ? (
          Object.entries(data.network).slice(0, 2).map(([iface, net]) => (
            <div key={iface} className="net-row">
              <span className="net-iface">{iface}</span>
              <span className="net-rx">↓{formatRate(net.rxRate)}</span>
              <span className="net-tx">↑{formatRate(net.txRate)}</span>
            </div>
          ))
        ) : (
          <div className="sub">No active interfaces</div>
        )}
      </div>
    </div>
  );
}

export default App;
`
}

func appCSS() string {
	return `:root {
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

html, body, #root {
  width: 480px;
  height: 320px;
  overflow: hidden;
}

body {
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
  background: var(--bg-color);
  color: var(--text-primary);
}

.status {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  font-size: 18px;
  text-align: center;
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

.net-row {
  display: flex;
  gap: 8px;
  font-size: 11px;
  margin-top: 4px;
}

.net-iface {
  color: var(--text-secondary);
  min-width: 50px;
}

.net-rx { color: #00ff88; }
.net-tx { color: #ff8800; }
`
}

func viteEnvDTS() string {
	return `/// <reference types="vite/client" />
`
}

// SDK files

func sdkTypesTS() string {
	// Theme SDK types - designed for theme developers.
	// These types represent the transformed data that themes receive.
	// The client.ts transforms raw sensor data to this format.
	return `/** Sensor data provided to themes */
export interface SensorData {
  cpu: CpuData;
  gpu: GpuData;
  memory: MemoryData;
  disk: Record<string, DiskData>;
  network: Record<string, NetworkData>;
  timestamp?: number;
}

/** CPU sensor data */
export interface CpuData {
  /** CPU load percentage (0-100) */
  load: number;
  /** CPU temperature in Celsius */
  temperature?: number;
  /** CPU frequency in MHz */
  frequency?: number;
  /** Number of CPU cores */
  cores?: number;
}

/** GPU sensor data (NVIDIA or AMD) */
export interface GpuData {
  /** Whether a GPU is available */
  available: boolean;
  /** GPU name/model */
  name?: string;
  /** GPU temperature in Celsius */
  temperature?: number;
  /** GPU load percentage (0-100) */
  load?: number;
  /** Memory used in MB */
  memoryUsed?: number;
  /** Total memory in MB */
  memoryTotal?: number;
  /** Power draw in Watts */
  power?: number;
}

/** Memory sensor data */
export interface MemoryData {
  /** Used memory in MB */
  used: number;
  /** Total memory in MB */
  total: number;
  /** Available memory in MB */
  available: number;
  /** Memory usage percentage (0-100) */
  percent: number;
}

/** Disk sensor data */
export interface DiskData {
  /** Mount point path */
  mountpoint: string;
  /** Used space in GB */
  used: number;
  /** Total space in GB */
  total: number;
  /** Free space in GB */
  free: number;
  /** Usage percentage (0-100) */
  percent: number;
}

/** Network interface sensor data */
export interface NetworkData {
  /** Interface name */
  interface: string;
  /** Receive rate in bytes/sec */
  rxRate: number;
  /** Transmit rate in bytes/sec */
  txRate: number;
}

export type ConnectionStatus = "connecting" | "connected" | "disconnected" | "error";
`
}

func sdkClientTS() string {
	return `import type { SensorData, ConnectionStatus } from "./types";

const DEFAULT_PORTS = [19847, 19848, 19849, 19850, 19851];
const RECONNECT_DELAY = 2000;

type DataListener = (data: SensorData) => void;
type StatusListener = (status: ConnectionStatus) => void;

// Transform raw JSON data from the server to our SensorData format
function transformData(raw: Record<string, unknown>): SensorData {
  const cpu = raw.cpu as Record<string, unknown> | undefined;
  const memory = raw.memory as Record<string, unknown> | undefined;
  const disk = raw.disk as Record<string, unknown> | undefined;
  const network = raw.network as Record<string, unknown> | undefined;
  
  // GPU: prefer nvidia_gpu, fall back to amd_gpu
  const nvidiaGpu = raw.nvidia_gpu as Record<string, unknown> | undefined;
  const amdGpu = raw.amd_gpu as Record<string, unknown> | undefined;
  const gpu = nvidiaGpu ?? amdGpu;

  // Disk: extract _items array
  const diskItems = disk?._items as Array<Record<string, unknown>> | undefined;
  const diskMap: Record<string, SensorData["disk"][string]> = {};
  if (diskItems) {
    for (const d of diskItems) {
      const mp = String(d.mount ?? "/");
      diskMap[mp] = {
        mountpoint: mp,
        used: Number(d.used ?? 0),
        total: Number(d.total ?? 0),
        free: Number(d.free ?? 0),
        percent: Number(d.percent ?? 0),
      };
    }
  }

  // Network: extract _items array
  const networkItems = network?._items as Array<Record<string, unknown>> | undefined;
  const networkMap: Record<string, SensorData["network"][string]> = {};
  if (networkItems) {
    for (const n of networkItems) {
      const iface = String(n.interface ?? "unknown");
      networkMap[iface] = {
        interface: iface,
        rxRate: Number(n.rx_rate ?? 0),
        txRate: Number(n.tx_rate ?? 0),
      };
    }
  }

  return {
    cpu: {
      temperature: cpu?.temperature as number | undefined,
      load: Number(cpu?.load ?? 0),
      frequency: cpu?.frequency as number | undefined,
      cores: cpu?.cores as number | undefined,
    },
    gpu: {
      available: Boolean(gpu),
      name: gpu?.name as string | undefined,
      temperature: gpu?.temperature as number | undefined,
      load: gpu?.load as number | undefined,
      memoryUsed: gpu?.memory_used as number | undefined,
      memoryTotal: gpu?.memory_total as number | undefined,
      power: gpu?.power as number | undefined,
    },
    memory: {
      used: Number(memory?.used ?? 0),
      total: Number(memory?.total ?? 0),
      available: Number(memory?.available ?? 0),
      percent: Number(memory?.percent ?? 0),
    },
    disk: diskMap,
    network: networkMap,
    timestamp: Date.now(),
  };
}

class SensorPanelClient {
  private ws: WebSocket | null = null;
  private dataListeners = new Set<DataListener>();
  private statusListeners = new Set<StatusListener>();
  private status: ConnectionStatus = "disconnected";
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null;
  private currentPortIndex = 0;
  private postMessageBound = false;

  constructor() {
    // Listen for postMessage events (used in headless browser mode)
    this.setupPostMessageListener();
  }

  private setupPostMessageListener(): void {
    if (this.postMessageBound) return;
    this.postMessageBound = true;

    window.addEventListener("message", (event: MessageEvent) => {
      if (event.data?.type === "sensorData") {
        const raw = event.data.data as Record<string, unknown>;
        const data = transformData(raw);
        this.dataListeners.forEach((fn) => fn(data));
        // If we receive postMessage data, we're connected via postMessage
        if (this.status !== "connected") {
          this.setStatus("connected");
        }
      }
    });
  }

  async connect(): Promise<void> {
    this.setStatus("connecting");

    // Check for ?ws=PORT query param first
    const params = new URLSearchParams(window.location.search);
    const wsPortParam = params.get("ws");
    
    if (wsPortParam) {
      const port = parseInt(wsPortParam, 10);
      if (!isNaN(port)) {
        try {
          await this.tryConnect(port);
          return;
        } catch {
          // Fall through to auto-discovery
        }
      }
    }

    for (let i = 0; i < DEFAULT_PORTS.length; i++) {
      const port = DEFAULT_PORTS[(this.currentPortIndex + i) % DEFAULT_PORTS.length]!;
      try {
        await this.tryConnect(port);
        this.currentPortIndex = DEFAULT_PORTS.indexOf(port);
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
      const wsHost = window.location.hostname || "localhost";
      const ws = new WebSocket(` + "`ws://${wsHost}:${port}/ws`" + `);
      const timeout = setTimeout(() => {
        ws.close();
        reject(new Error("Connection timeout"));
      }, 1000);

      ws.onopen = () => {
        clearTimeout(timeout);
        this.ws = ws;
        this.setStatus("connected");
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

    this.ws.onmessage = (event: MessageEvent<string>) => {
      try {
        const raw = JSON.parse(event.data) as Record<string, unknown>;
        const data = transformData(raw);
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
    this.reconnectTimer = setTimeout(() => {
      this.reconnectTimer = null;
      void this.connect();
    }, RECONNECT_DELAY);
  }

  subscribe(callback: DataListener): () => void {
    this.dataListeners.add(callback);
    return () => { this.dataListeners.delete(callback); };
  }

  onStatusChange(callback: StatusListener): () => void {
    this.statusListeners.add(callback);
    callback(this.status);
    return () => { this.statusListeners.delete(callback); };
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
`
}

func sdkHooksTS() string {
	return `import { useState, useEffect } from "react";
import { client } from "./client";
import type { SensorData, ConnectionStatus, CpuData, GpuData, MemoryData } from "./types";

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

export function useMemoryData(): MemoryData | null {
  const data = useSensorData();
  return data?.memory ?? null;
}

export function formatBytes(bytes: number): string {
  if (bytes >= 1e9) return ` + "`${(bytes / 1e9).toFixed(1)} GB`" + `;
  if (bytes >= 1e6) return ` + "`${(bytes / 1e6).toFixed(1)} MB`" + `;
  if (bytes >= 1e3) return ` + "`${(bytes / 1e3).toFixed(1)} KB`" + `;
  return ` + "`${bytes} B`" + `;
}

export function formatRate(bytesPerSec: number): string {
  if (bytesPerSec >= 1e6) return ` + "`${(bytesPerSec / 1e6).toFixed(1)} MB/s`" + `;
  if (bytesPerSec >= 1e3) return ` + "`${(bytesPerSec / 1e3).toFixed(0)} KB/s`" + `;
  return ` + "`${bytesPerSec} B/s`" + `;
}
`
}

func sdkIndexTS() string {
	return `// Client
export { client, SensorPanelClient } from "./client";

// Hooks
export {
  useSensorData,
  useConnectionStatus,
  useCpuData,
  useGpuData,
  useMemoryData,
  formatBytes,
  formatRate,
} from "./hooks";

// Types
export type {
  SensorData,
  CpuData,
  GpuData,
  MemoryData,
  DiskData,
  NetworkData,
  ConnectionStatus,
} from "./types";
`
}

// distIndexHTML creates a pre-built standalone HTML file that works without bundling.
// This allows themes to work immediately after creation without running npm build.
func distIndexHTML(name string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8" />
  <meta name="viewport" content="width=480, height=320, initial-scale=1.0" />
  <title>%s - SensorPanel</title>
  <style>
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
    * { margin: 0; padding: 0; box-sizing: border-box; }
    html, body { width: 480px; height: 320px; overflow: hidden; font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif; background: var(--bg-color); color: var(--text-primary); }
    .status { display: flex; flex-direction: column; align-items: center; justify-content: center; height: 100%%; font-size: 18px; text-align: center; }
    .status.error { color: var(--accent-cpu); }
    .status small { margin-top: 8px; color: var(--text-secondary); font-size: 12px; }
    .dashboard { display: grid; grid-template-columns: repeat(2, 1fr); grid-template-rows: repeat(2, 1fr); gap: 8px; padding: 8px; height: 100%%; }
    .metric { background: var(--card-bg); border-radius: 8px; padding: 12px; display: flex; flex-direction: column; justify-content: center; }
    .metric .label { font-size: 14px; color: var(--text-secondary); text-transform: uppercase; letter-spacing: 1px; }
    .metric .value { font-size: 36px; font-weight: bold; margin: 4px 0; }
    .metric .sub { font-size: 12px; color: var(--text-secondary); }
    .metric.cpu { border-left: 4px solid var(--accent-cpu); }
    .metric.gpu { border-left: 4px solid var(--accent-gpu); }
    .metric.ram { border-left: 4px solid var(--accent-ram); }
    .metric.network { border-left: 4px solid var(--accent-network); }
    .net-row { display: flex; gap: 8px; font-size: 11px; margin-top: 4px; }
    .net-iface { color: var(--text-secondary); min-width: 50px; }
    .net-rx { color: #00ff88; }
    .net-tx { color: #ff8800; }
  </style>
</head>
<body>
  <div id="app" class="status">Connecting to SensorPanel...</div>

  <script>
    const DEFAULT_PORTS = [19847, 19848, 19849, 19850, 19851];
    const RECONNECT_DELAY = 2000;
    let ws = null;
    let reconnectTimer = null;
    let currentPortIndex = 0;

    const fmtRate = (b) => {
      if (b >= 1e6) return (b / 1e6).toFixed(1) + ' MB/s';
      if (b >= 1e3) return (b / 1e3).toFixed(0) + ' KB/s';
      return b + ' B/s';
    };

    function render(data) {
      const app = document.getElementById('app');
      
      const networks = (data.networks || [])
        .filter(n => n.rx_bytes_per_sec > 0 || n.tx_bytes_per_sec > 0)
        .slice(0, 2)
        .map(n => `+"`<div class=\"net-row\"><span class=\"net-iface\">${n.interface}</span><span class=\"net-rx\">↓${fmtRate(n.rx_bytes_per_sec)}</span><span class=\"net-tx\">↑${fmtRate(n.tx_bytes_per_sec)}</span></div>`"+`)
        .join('');

      app.className = 'dashboard';
      app.innerHTML = `+"`"+`
        <div class="metric cpu">
          <div class="label">CPU</div>
          <div class="value">${(data.cpu?.load_percent ?? 0).toFixed(0)}%%</div>
          <div class="sub">${data.cpu?.temperature?.toFixed(0) ?? '--'}°C</div>
        </div>
        <div class="metric gpu">
          <div class="label">GPU</div>
          <div class="value">${(data.gpu?.load_percent ?? 0).toFixed(0)}%%</div>
          <div class="sub">${data.gpu?.temperature?.toFixed(0) ?? '--'}°C</div>
        </div>
        <div class="metric ram">
          <div class="label">RAM</div>
          <div class="value">${(data.memory?.percent ?? 0).toFixed(0)}%%</div>
          <div class="sub">${((data.memory?.used_mb ?? 0) / 1024).toFixed(1)} / ${((data.memory?.total_mb ?? 0) / 1024).toFixed(1)} GB</div>
        </div>
        <div class="metric network">
          <div class="label">NET</div>
          ${networks || '<div class="sub">No active interfaces</div>'}
        </div>
      `+"`"+`;
    }

    function showError(msg) {
      const app = document.getElementById('app');
      app.className = 'status error';
      app.innerHTML = msg + '<br><small>Make sure \'sensorpanel theme dev\' is running</small>';
    }

    async function tryConnect(port) {
      return new Promise((resolve, reject) => {
        const wsHost = location.hostname || 'localhost';
        const socket = new WebSocket(`+"`ws://${wsHost}:${port}/ws`"+`);
        const timeout = setTimeout(() => {
          socket.close();
          reject(new Error('timeout'));
        }, 1000);

        socket.onopen = () => {
          clearTimeout(timeout);
          ws = socket;
          setupListeners();
          resolve();
        };

        socket.onerror = () => {
          clearTimeout(timeout);
          reject(new Error('failed'));
        };
      });
    }

    function setupListeners() {
      if (!ws) return;
      ws.onmessage = (e) => {
        try { render(JSON.parse(e.data)); } catch {}
      };
      ws.onclose = () => scheduleReconnect();
      ws.onerror = () => {};
    }

    function scheduleReconnect() {
      if (reconnectTimer) return;
      showError('Disconnected. Retrying...');
      reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        void connect();
      }, RECONNECT_DELAY);
    }

    async function connect() {
      // Check for ?ws=PORT
      const params = new URLSearchParams(location.search);
      const wsPort = params.get('ws');
      if (wsPort) {
        try {
          await tryConnect(parseInt(wsPort, 10));
          return;
        } catch {}
      }

      for (let i = 0; i < DEFAULT_PORTS.length; i++) {
        const port = DEFAULT_PORTS[(currentPortIndex + i) %% DEFAULT_PORTS.length];
        try {
          await tryConnect(port);
          currentPortIndex = DEFAULT_PORTS.indexOf(port);
          return;
        } catch {}
      }

      showError('Connection failed');
      scheduleReconnect();
    }

    // Also accept postMessage for preview mode
    window.addEventListener('message', (e) => {
      if (e.data?.type === 'sensorData') render(e.data.data);
    });

    connect();
  </script>
</body>
</html>
`, name)
}
