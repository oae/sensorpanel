import { useState, useEffect } from "react";
import { client } from "./client";
import type { SensorData, ConnectionStatus, CpuData, GpuData, MemoryData, MotherboardData, HostnameData } from "./types";

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
  return data?.amdGpu ?? data?.nvidiaGpu ?? null;
}

export function useMemoryData(): MemoryData | null {
  const data = useSensorData();
  return data?.memory ?? null;
}

export function useMotherboardData(): MotherboardData | null {
  const data = useSensorData();
  return data?.motherboard ?? null;
}

export function useHostnameData(): HostnameData | null {
  const data = useSensorData();
  return data?.hostname ?? null;
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
