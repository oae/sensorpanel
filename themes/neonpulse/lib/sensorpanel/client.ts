import type { SensorData, ConnectionStatus } from "./types";

const DEFAULT_PORTS = [19847, 19848, 19849, 19850, 19851];
const RECONNECT_DELAY = 2000;

type DataListener = (data: SensorData) => void;
type StatusListener = (status: ConnectionStatus) => void;

// Transform raw JSON data from the server to our SensorData format
function transformData(raw: Record<string, unknown>): SensorData {
  const result: SensorData = {
    timestamp: Date.now(),
  };

  // CPU
  const cpu = raw.cpu as Record<string, unknown> | undefined;
  if (cpu) {
    result.cpu = {
      name: String(cpu.name ?? "CPU"),
      load: Number(cpu.load ?? 0),
      temperature: cpu.temperature as number | undefined,
      frequency: cpu.frequency as number | undefined,
      cores: Number(cpu.cores ?? 0),
    };
  }

  // Memory
  const memory = raw.memory as Record<string, unknown> | undefined;
  if (memory) {
    result.memory = {
      used: Number(memory.used ?? 0),
      total: Number(memory.total ?? 0),
      available: Number(memory.available ?? 0),
      percent: Number(memory.percent ?? 0),
    };
  }

  // AMD GPU
  const amdGpu = raw.amd_gpu as Record<string, unknown> | undefined;
  if (amdGpu) {
    result.amdGpu = {
      name: String(amdGpu.name ?? "AMD GPU"),
      temperature: amdGpu.temperature as number | undefined,
      load: amdGpu.load as number | undefined,
      memoryUsed: amdGpu.memory_used as number | undefined,
      memoryTotal: amdGpu.memory_total as number | undefined,
      power: amdGpu.power as number | undefined,
      fanSpeed: amdGpu.fan_speed as number | undefined,
      voltage: amdGpu.voltage as number | undefined,
      clock: amdGpu.clock as number | undefined,
      memoryClock: amdGpu.memory_clock as number | undefined,
    };
  }

  // NVIDIA GPU
  const nvidiaGpu = raw.nvidia_gpu as Record<string, unknown> | undefined;
  if (nvidiaGpu) {
    result.nvidiaGpu = {
      name: String(nvidiaGpu.name ?? "NVIDIA GPU"),
      temperature: nvidiaGpu.temperature as number | undefined,
      load: nvidiaGpu.load as number | undefined,
      memoryUsed: nvidiaGpu.memory_used as number | undefined,
      memoryTotal: nvidiaGpu.memory_total as number | undefined,
      power: nvidiaGpu.power as number | undefined,
      fanSpeed: nvidiaGpu.fan_speed as number | undefined,
      clock: nvidiaGpu.clock as number | undefined,
      memoryClock: nvidiaGpu.memory_clock as number | undefined,
    };
  }

  // Motherboard
  const motherboard = raw.motherboard as Record<string, unknown> | undefined;
  if (motherboard) {
    result.motherboard = {
      cpuFan: motherboard.cpu_fan as number | undefined,
      chipsetFan: motherboard.chipset_fan as number | undefined,
      systemFan1: motherboard.system_fan1 as number | undefined,
      systemFan2: motherboard.system_fan2 as number | undefined,
      systemFan3: motherboard.system_fan3 as number | undefined,
      cpuVoltage: motherboard.cpu_voltage as number | undefined,
      dimm1Temp: motherboard.dimm1_temp as number | undefined,
      dimm2Temp: motherboard.dimm2_temp as number | undefined,
      dimm3Temp: motherboard.dimm3_temp as number | undefined,
      dimm4Temp: motherboard.dimm4_temp as number | undefined,
    };
  }

  // Hostname
  const hostname = raw.hostname as Record<string, unknown> | undefined;
  if (hostname) {
    result.hostname = {
      hostname: String(hostname.hostname ?? "Unknown"),
    };
  }

  // Disk: extract _items array
  const disk = raw.disk as Record<string, unknown> | undefined;
  const diskItems = disk?._items as Array<Record<string, unknown>> | undefined;
  if (diskItems && diskItems.length > 0) {
    result.disk = {};
    for (const d of diskItems) {
      const mp = String(d.mount ?? "/");
      result.disk[mp] = {
        mount: mp,
        label: String(d.label ?? mp),
        used: Number(d.used ?? 0),
        total: Number(d.total ?? 0),
        free: Number(d.free ?? 0),
        percent: Number(d.percent ?? 0),
      };
    }
  }

  // Network: extract _items array
  const network = raw.network as Record<string, unknown> | undefined;
  const networkItems = network?._items as Array<Record<string, unknown>> | undefined;
  if (networkItems && networkItems.length > 0) {
    result.network = {};
    for (const n of networkItems) {
      const iface = String(n.interface ?? "unknown");
      result.network[iface] = {
        interface: iface,
        rxRate: Number(n.rx_rate ?? 0),
        txRate: Number(n.tx_rate ?? 0),
        rxTotal: Number(n.rx_total ?? 0),
        txTotal: Number(n.tx_total ?? 0),
      };
    }
  }

  return result;
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
      const ws = new WebSocket(`ws://${wsHost}:${port}/ws`);
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
