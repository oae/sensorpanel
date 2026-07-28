/** AMD GPU statistics via sysfs */
export interface AmdGpuData {
  /** GPU name */
  name: string;
  /** GPU temperature (°C) */
  temperature?: number;
  /** GPU utilization (%) */
  load?: number;
  /** VRAM used (MB) */
  memoryUsed?: number;
  /** VRAM total (MB) */
  memoryTotal?: number;
  /** Power draw (W) */
  power?: number;
  /** Fan speed (%) */
  fanSpeed?: number;
  /** GPU core voltage (V) */
  voltage?: number;
  /** GPU clock speed (MHz) */
  clock?: number;
  /** Memory clock speed (MHz) */
  memoryClock?: number;
}

/** CPU usage, temperature, and frequency */
export interface CpuData {
  /** CPU model name */
  name: string;
  /** CPU load percentage (%) */
  load: number;
  /** CPU temperature (°C) */
  temperature?: number;
  /** CPU frequency (MHz) */
  frequency?: number;
  /** Number of CPU cores */
  cores: number;
}

/** Disk usage statistics */
export interface DiskData {
  /** Mount point */
  mount: string;
  /** Display label (alias or mount point) */
  label: string;
  /** Total disk space (GB) */
  total: number;
  /** Used disk space (GB) */
  used: number;
  /** Free disk space (GB) */
  free: number;
  /** Disk usage percentage (%) */
  percent: number;
}

/** System hostname and device name */
export interface HostnameData {
  /** System hostname */
  hostname: string;
}

/** System memory (RAM) usage */
export interface MemoryData {
  /** Total memory (MB) */
  total: number;
  /** Used memory (MB) */
  used: number;
  /** Available memory (MB) */
  available: number;
  /** Memory usage percentage (%) */
  percent: number;
}

/** Motherboard sensors (fans, voltages, temperatures) */
export interface MotherboardData {
  /** CPU fan speed (RPM) */
  cpuFan?: number;
  /** Chipset fan speed (RPM) */
  chipsetFan?: number;
  /** System fan 1 speed (RPM) */
  systemFan1?: number;
  /** System fan 2 speed (RPM) */
  systemFan2?: number;
  /** System fan 3 speed (RPM) */
  systemFan3?: number;
  /** CPU core voltage (V) */
  cpuVoltage?: number;
  /** DIMM 1 temperature (°C) */
  dimm1Temp?: number;
  /** DIMM 2 temperature (°C) */
  dimm2Temp?: number;
  /** DIMM 3 temperature (°C) */
  dimm3Temp?: number;
  /** DIMM 4 temperature (°C) */
  dimm4Temp?: number;
}

/** Network interface statistics */
export interface NetworkData {
  /** Interface name */
  interface: string;
  /** Receive rate (B/s) */
  rxRate: number;
  /** Transmit rate (B/s) */
  txRate: number;
  /** Total bytes received (bytes) */
  rxTotal: number;
  /** Total bytes transmitted (bytes) */
  txTotal: number;
}

/** NVIDIA GPU statistics via nvidia-smi */
export interface NvidiaGpuData {
  /** GPU name */
  name: string;
  /** GPU temperature (°C) */
  temperature?: number;
  /** GPU utilization (%) */
  load?: number;
  /** VRAM used (MB) */
  memoryUsed?: number;
  /** VRAM total (MB) */
  memoryTotal?: number;
  /** Power draw (W) */
  power?: number;
  /** Fan speed (%) */
  fanSpeed?: number;
  /** GPU clock speed (MHz) */
  clock?: number;
  /** Memory clock speed (MHz) */
  memoryClock?: number;
}

export interface SensorData {
  amdGpu?: AmdGpuData;
  cpu?: CpuData;
  disk?: Record<string, DiskData>;
  hostname?: HostnameData;
  memory?: MemoryData;
  motherboard?: MotherboardData;
  network?: Record<string, NetworkData>;
  nvidiaGpu?: NvidiaGpuData;
  timestamp?: number;
}

/** Unified GPU data (either AMD or NVIDIA) */
export type GpuData = AmdGpuData | NvidiaGpuData;

export type ConnectionStatus = "connecting" | "connected" | "disconnected" | "error";
