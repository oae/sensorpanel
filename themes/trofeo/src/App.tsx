import { useEffect, useState } from "react";
import { useSensorData } from "../lib/sensorpanel";
import "./App.css";

type Point = { label: string; value: string; percent: number; tone?: "blue" | "green" | "amber" | "red" | "violet" };

function App() {
  const data = useSensorData();
  const [time, setTime] = useState(new Date());

  useEffect(() => {
    let interval: number | undefined;
    const timeout = window.setTimeout(() => {
      setTime(new Date());
      interval = window.setInterval(() => setTime(new Date()), 60_000);
    }, 60_000 - (Date.now() % 60_000));

    return () => {
      window.clearTimeout(timeout);
      if (interval !== undefined) {
        window.clearInterval(interval);
      }
    };
  }, []);

  if (!data) {
    return <div className="loading" aria-label="Waiting for sensor data" />;
  }

  const gpu = data.nvidiaGpu ?? data.amdGpu;
  const disks = Object.entries(data.disk ?? {}).slice(0, 3);
  const networkEntries = Object.entries(data.network ?? {});
  const primaryNet = networkEntries.find(([, n]) => n.rxRate > 0 || n.txRate > 0)?.[1] ?? networkEntries[0]?.[1];
  const cpuFanRpm = data.motherboard?.systemFan2 ?? data.motherboard?.cpuFan ?? 0;

  const cpuPoints: Point[] = [
    { label: "LOAD", value: `${data.cpu?.load?.toFixed(0) ?? "--"}%`, percent: data.cpu?.load ?? 0, tone: "blue" },
    { label: "CLOCK", value: `${((data.cpu?.frequency ?? 0) / 1000).toFixed(2)} GHz`, percent: ((data.cpu?.frequency ?? 0) / 6000) * 100, tone: "violet" },
    { label: "FAN", value: `${cpuFanRpm.toFixed(0)} RPM`, percent: (cpuFanRpm / 5000) * 100, tone: "green" },
  ];

  const gpuPoints: Point[] = [
    { label: "LOAD", value: `${gpu?.load?.toFixed(0) ?? "--"}%`, percent: gpu?.load ?? 0, tone: "blue" },
    { label: "CLOCK", value: `${gpu?.clock?.toFixed(0) ?? "--"} MHz`, percent: ((gpu?.clock ?? 0) / 3000) * 100, tone: "violet" },
    { label: "VRAM", value: `${((gpu?.memoryUsed ?? 0) / 1024).toFixed(1)} GB`, percent: ((gpu?.memoryUsed ?? 0) / (gpu?.memoryTotal ?? 1)) * 100, tone: "amber" },
    { label: "POWER", value: `${gpu?.power?.toFixed(0) ?? "--"} W`, percent: ((gpu?.power ?? 0) / 450) * 100, tone: "green" },
  ];

  const memPercent = data.memory?.percent ?? 0;
  const hostname = data.hostname?.hostname ?? "SensorPanel";

  return (
    <main className="dashboard">
      <section className="hero">
        <div className="topline">
          <div>
            <div className="eyebrow">Thermalright Trofeo 9.16</div>
            <h1>{hostname}</h1>
          </div>
          <div className="clock">
            <strong>{time.toLocaleTimeString("en-US", { hour: "2-digit", minute: "2-digit", hour12: false })}</strong>
            <span>{time.toLocaleDateString("en-US", { weekday: "short", month: "short", day: "numeric" })}</span>
          </div>
        </div>

        <div className="thermals">
          <ThermalGauge label="CPU" name={data.cpu?.name ?? "Processor"} temp={data.cpu?.temperature} load={data.cpu?.load} tone="cpu" />
          <ThermalGauge label="GPU" name={gpu?.name ?? "Graphics"} temp={gpu?.temperature} load={gpu?.load} tone="gpu" />
        </div>
      </section>

      <section className="metrics">
        <MetricGroup title="CPU" points={cpuPoints} />
        <MetricGroup title="GPU" points={gpuPoints} />
      </section>

      <section className="memory">
        <div className="section-title">MEMORY</div>
        <div className="memory-main">
          <strong>{memPercent.toFixed(0)}%</strong>
          <span>{((data.memory?.used ?? 0) / 1024).toFixed(1)} / {((data.memory?.total ?? 0) / 1024).toFixed(0)} GB</span>
        </div>
        <Bar percent={memPercent} tone="violet" />
        <div className="dimm-row">
          <Dimm label="A1" value={data.motherboard?.dimm1Temp} />
          <Dimm label="A2" value={data.motherboard?.dimm3Temp} />
          <Dimm label="B1" value={data.motherboard?.dimm2Temp} />
          <Dimm label="B2" value={data.motherboard?.dimm4Temp} />
        </div>
      </section>

      <section className="io">
        <div className="section-title">STORAGE / NETWORK</div>
        <div className="disk-list">
          {disks.map(([mount, disk]) => (
            <div className="disk" key={mount}>
              <div className="disk-head">
                <span>{disk.label || mount}</span>
                <strong>{disk.percent.toFixed(0)}%</strong>
              </div>
              <Bar percent={disk.percent} tone="green" />
            </div>
          ))}
        </div>
        <div className="net">
          <span>DOWN <strong>{formatSpeed(primaryNet?.rxRate ?? 0)}</strong></span>
          <span>UP <strong>{formatSpeed(primaryNet?.txRate ?? 0)}</strong></span>
        </div>
      </section>
    </main>
  );
}

function ThermalGauge({ label, name, temp, load, tone }: { label: string; name: string; temp?: number; load?: number; tone: "cpu" | "gpu" }) {
  const safeTemp = temp ?? 0;
  const circumference = 2 * Math.PI * 78;
  const dash = (Math.min(100, safeTemp) / 100) * circumference;

  return (
    <div className={`thermal ${tone}`}>
      <svg viewBox="0 0 190 190" aria-hidden="true">
        <circle className="ring-bg" cx="95" cy="95" r="78" />
        <circle className="ring-fg" cx="95" cy="95" r="78" strokeDasharray={`${dash} ${circumference}`} />
      </svg>
      <div className="thermal-text">
        <span>{label}</span>
        <strong>{temp?.toFixed(0) ?? "--"}°</strong>
        <em>{load?.toFixed(0) ?? "--"}% load</em>
      </div>
      <div className="device-name" title={name}>{name}</div>
    </div>
  );
}

function MetricGroup({ title, points }: { title: string; points: Point[] }) {
  return (
    <div className="metric-group">
      <div className="section-title">{title}</div>
      {points.map((point) => (
        <div className="metric" key={point.label}>
          <div className="metric-label">
            <span>{point.label}</span>
            <strong>{point.value}</strong>
          </div>
          <Bar percent={point.percent} tone={point.tone ?? "blue"} />
        </div>
      ))}
    </div>
  );
}

function Dimm({ label, value }: { label: string; value?: number | null }) {
  return (
    <div className="dimm">
      <span>{label}</span>
      <strong>{value == null ? "--" : `${value.toFixed(0)}°`}</strong>
    </div>
  );
}

function Bar({ percent, tone }: { percent: number; tone: Point["tone"] }) {
  const width = `${Math.max(0, Math.min(100, percent))}%`;
  return (
    <div className={`bar ${tone}`}>
      <div style={{ width }} />
    </div>
  );
}

function formatSpeed(bytesPerSec: number): string {
  if (bytesPerSec >= 1024 * 1024) return `${(bytesPerSec / 1024 / 1024).toFixed(1)} MB/s`;
  if (bytesPerSec >= 1024) return `${(bytesPerSec / 1024).toFixed(0)} KB/s`;
  return `${bytesPerSec.toFixed(0)} B/s`;
}

export default App;
