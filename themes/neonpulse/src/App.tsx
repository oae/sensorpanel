import { useSensorData, useConnectionStatus } from "../lib/sensorpanel";
import "./App.css";
import { useEffect, useRef, useState } from "react";

// Temperature history for graphs
interface TempHistory {
  cpu: number[];
  gpu: number[];
}

function App() {
  const data = useSensorData();
  const status = useConnectionStatus();
  const [time, setTime] = useState(new Date());
  const [tempHistory, setTempHistory] = useState<TempHistory>({ cpu: [], gpu: [] });
  const maxHistoryLength = 60;

  // Update time every second
  useEffect(() => {
    const interval = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(interval);
  }, []);

  // Track temperature history
  useEffect(() => {
    if (!data) return;
    const cpuTemp = data.cpu?.temperature ?? 0;
    const gpuTemp = data.nvidiaGpu?.temperature ?? data.amdGpu?.temperature ?? 0;
    
    setTempHistory(prev => ({
      cpu: [...prev.cpu.slice(-maxHistoryLength + 1), cpuTemp],
      gpu: [...prev.gpu.slice(-maxHistoryLength + 1), gpuTemp],
    }));
  }, [data?.timestamp]);

  if (status === "connecting") {
    return <div className="loading">Connecting...</div>;
  }

  if (status === "error" || status === "disconnected") {
    return <div className="loading error">Disconnected. Retrying...</div>;
  }

  if (!data) {
    return <div className="loading">Waiting for data...</div>;
  }

  // Prefer NVIDIA (discrete) over AMD (often integrated)
  const gpu = data.nvidiaGpu ?? data.amdGpu;
  const hostname = data.hostname?.hostname ?? "My PC";
  // Use system_fan2 for CPU fan (likely AIO pump based on high RPM)
  const cpuFanRpm = data.motherboard?.systemFan2 ?? data.motherboard?.cpuFan ?? 0;

  // Get disks for storage display (max 3)
  const disks = Object.entries(data.disk ?? {}).slice(0, 3);

  // Get primary network interface (first one with traffic)
  const networkEntries = Object.entries(data.network ?? {});
  const primaryNet = networkEntries.find(([, n]) => n.rxRate > 0 || n.txRate > 0)?.[1] ?? networkEntries[0]?.[1];

  // Format network speed
  const formatSpeed = (bytesPerSec: number): string => {
    if (bytesPerSec >= 1024 * 1024) return `${(bytesPerSec / 1024 / 1024).toFixed(1)} MB/s`;
    if (bytesPerSec >= 1024) return `${(bytesPerSec / 1024).toFixed(0)} KB/s`;
    return `${bytesPerSec.toFixed(0)} B/s`;
  };

  return (
    <div className="dashboard">
      {/* Row 1: Device Info + Time + Storage */}
      <div className="panel device-panel">
        <div className="device-top">
          <div className="device-info">
            <div className="panel-title">Device Name:</div>
            <div className="device-name">{hostname}</div>
            <div className="status-row">
              <span className="status-online">Online</span>
              {primaryNet && (
                <span className="network-speed">
                  <span className="net-down">↓{formatSpeed(primaryNet.rxRate)}</span>
                  <span className="net-up">↑{formatSpeed(primaryNet.txRate)}</span>
                </span>
              )}
            </div>
          </div>
          <div className="time-section">
            <div className="time">{time.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: true })}</div>
            <div className="date">{time.toLocaleDateString('en-US', { month: 'numeric', day: 'numeric', year: 'numeric' })}</div>
          </div>
        </div>
        <div className="storage-list">
          {disks.map(([mount, disk]) => (
            <div key={mount} className="storage-inline" title={mount}>
              <span className="storage-label">{disk.label}</span>
              <span className="storage-percent">{disk.percent.toFixed(0)}%</span>
              <div className="storage-bar">
                <div className="bar-fill" style={{ width: `${disk.percent}%` }} />
              </div>
              <span className="storage-free">{disk.free.toFixed(0)} GB</span>
            </div>
          ))}
        </div>
      </div>

      {/* CPU Panel - top right */}
      <div className="panel cpu-panel">
        <div className="cpu-header">
          <div className="cpu-name">{data.cpu?.name ?? "CPU"}</div>
        </div>
        <div className="cpu-content">
          <div className="temp-ring cpu-ring">
            <svg viewBox="0 0 100 100">
              <circle className="ring-bg" cx="50" cy="50" r="40" />
              <circle 
                className="ring-value cpu-ring-value" 
                cx="50" cy="50" r="40" 
                strokeDasharray={`${(data.cpu?.temperature ?? 0) / 100 * 251.2} 251.2`}
              />
            </svg>
            <div className="ring-text">{data.cpu?.temperature?.toFixed(0) ?? "--"}°C</div>
          </div>
          <div className="cpu-stats">
            <StatRow label="Load" value={`${data.cpu?.load?.toFixed(0) ?? 0}%`} percent={data.cpu?.load ?? 0} color="gradient-blue" />
            <StatRow label="CPU Clock" value={`${((data.cpu?.frequency ?? 0) / 1000).toFixed(2)}GHz`} percent={((data.cpu?.frequency ?? 0) / 6000) * 100} color="gradient-rainbow" />
            <StatRow label="CPU Fan" value={`${cpuFanRpm.toFixed(0)} RPM`} percent={(cpuFanRpm / 5000) * 100} color="gradient-rainbow" />
            <StatRow label="Cores" value={`${data.cpu?.cores ?? 0}`} percent={100} color="gradient-blue" />
          </div>
        </div>
        <TempGraph data={tempHistory.cpu} minTemp={40} maxTemp={95} color="#00bfff" />
      </div>

      {/* GPU Panel - left side, spans 2 rows */}
      <div className="panel gpu-panel">
        <div className="gpu-name" title={gpu?.name ?? "GPU"}>{gpu?.name ?? "GPU"}</div>
        <div className="gpu-content">
          <div className="temp-ring gpu-ring">
            <svg viewBox="0 0 100 100">
              <circle className="ring-bg" cx="50" cy="50" r="40" />
              <circle 
                className="ring-value gpu-ring-value" 
                cx="50" cy="50" r="40" 
                strokeDasharray={`${(gpu?.temperature ?? 0) / 100 * 251.2} 251.2`}
              />
            </svg>
            <div className="ring-text">{gpu?.temperature?.toFixed(0) ?? "--"}°C</div>
          </div>
          <div className="gpu-stats">
            <StatRow label="Load" value={`${gpu?.load?.toFixed(0) ?? 0}%`} percent={gpu?.load ?? 0} color="gradient-blue" />
            <StatRow label="GPU Clock" value={`${gpu?.clock?.toFixed(0) ?? 0} MHz`} percent={((gpu?.clock ?? 0) / 3000) * 100} color="gradient-blue" />
            <StatRow label="Mem Clock" value={`${gpu?.memoryClock?.toFixed(0) ?? 0} MHz`} percent={((gpu?.memoryClock ?? 0) / 10000) * 100} color="gradient-blue" />
            <StatRow label="VRAM" value={`${((gpu?.memoryUsed ?? 0) / 1024).toFixed(1)}GB`} percent={((gpu?.memoryUsed ?? 0) / (gpu?.memoryTotal ?? 1)) * 100} color="gradient-yellow" />
            <StatRow label="Fan" value={`${gpu?.fanSpeed?.toFixed(0) ?? 0}%`} percent={gpu?.fanSpeed ?? 0} color="gradient-blue" />
            <StatRow label="Power" value={`${gpu?.power?.toFixed(0) ?? 0}W`} percent={((gpu?.power ?? 0) / 450) * 100} color="gradient-rainbow" />
          </div>
        </div>
        <TempGraph data={tempHistory.gpu} minTemp={30} maxTemp={85} color="#00bfff" />
      </div>

      {/* Memory Panel - right side */}
      <div className="panel memory-panel">
        <div className="memory-title">Memory</div>
        <div className="memory-header">DDR5-6400 | {((data.memory?.total ?? 0) / 1024).toFixed(0)} GB</div>
        <div className="dimm-layout">
          <DimmTemp value={data.motherboard?.dimm1Temp ?? null} slot="A1" />
          <DimmTemp value={data.motherboard?.dimm3Temp ?? null} slot="A2" />
          <DimmTemp value={data.motherboard?.dimm2Temp ?? null} slot="B1" />
          <DimmTemp value={data.motherboard?.dimm4Temp ?? null} slot="B2" />
        </div>
        <div className="memory-stats">
          <div className="memory-usage">
            <span className="memory-label">Usage</span>
            <span className="memory-percent">{data.memory?.percent?.toFixed(0) ?? 0}%</span>
          </div>
          <div className="memory-bar">
            <div className="bar-fill" style={{ width: `${data.memory?.percent ?? 0}%` }} />
          </div>
          <div className="memory-details">
            <span>Used: {((data.memory?.used ?? 0) / 1024).toFixed(1)} GB</span>
            <span>Free: {((data.memory?.available ?? 0) / 1024).toFixed(1)} GB</span>
          </div>
        </div>
      </div>
    </div>
  );
}

function StatRow({ label, value, percent, color }: { label: string; value: string; percent: number; color: string }) {
  return (
    <div className="stat-row">
      <span className="stat-label">{label}</span>
      <span className="stat-value">{value}</span>
      <div className={`stat-bar ${color}`}>
        <div className="bar-fill" style={{ width: `${Math.min(100, percent)}%` }} />
      </div>
    </div>
  );
}

function TempGraph({ data, minTemp, maxTemp, color }: { data: number[]; minTemp: number; maxTemp: number; color: string }) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  
  useEffect(() => {
    const canvas = canvasRef.current;
    const container = containerRef.current;
    if (!canvas || !container) return;
    
    // Resize canvas to match container
    const rect = container.getBoundingClientRect();
    canvas.width = rect.width;
    canvas.height = rect.height;
    
    const ctx = canvas.getContext('2d');
    if (!ctx) return;
    
    const width = canvas.width;
    const height = canvas.height;
    const padding = 2;
    const drawHeight = height - padding * 2;
    
    ctx.clearRect(0, 0, width, height);
    
    if (data.length < 2) return;
    
    ctx.strokeStyle = color;
    ctx.lineWidth = 1.5;
    ctx.beginPath();
    
    const range = maxTemp - minTemp;
    const stepX = (width - padding * 2) / (data.length - 1);
    
    data.forEach((temp, i) => {
      const x = padding + i * stepX;
      const clampedTemp = Math.max(minTemp, Math.min(maxTemp, temp));
      const y = padding + drawHeight - ((clampedTemp - minTemp) / range) * drawHeight;
      if (i === 0) ctx.moveTo(x, y);
      else ctx.lineTo(x, y);
    });
    
    ctx.stroke();
  }, [data, minTemp, maxTemp, color]);
  
  return (
    <div className="temp-graph">
      <div className="graph-labels">
        <span>{maxTemp}</span>
        <span>{minTemp}</span>
      </div>
      <div className="graph-canvas-container" ref={containerRef}>
        <canvas ref={canvasRef} />
      </div>
    </div>
  );
}

function DimmTemp({ value, slot }: { value: number | null | undefined; slot: string }) {
  const hasValue = value !== null && value !== undefined;
  return (
    <div className={`dimm-slot ${hasValue ? 'active' : 'inactive'}`}>
      <div className="dimm-temp-circle">
        {hasValue ? Math.round(value) : '--'}
      </div>
      <div className="dimm-slot-label">{slot}</div>
    </div>
  );
}

export default App;
