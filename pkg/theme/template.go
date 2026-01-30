// Package theme - template files for new themes.
package theme

import "fmt"

// getTemplateFiles returns the template files for a new theme.
func getTemplateFiles(themeName string) map[string]string {
	return map[string]string{
		"package.json":               packageJSON(themeName),
		"vite.config.js":             viteConfig(),
		"index.html":                 indexHTML(themeName),
		"src/main.jsx":               mainJSX(),
		"src/App.jsx":                appJSX(),
		"src/App.css":                appCSS(),
		"src/hooks/useSensorData.js": useSensorDataHook(),
		"dist/index.html":            distIndexHTML(themeName),
	}
}

func packageJSON(name string) string {
	return fmt.Sprintf(`{
  "name": "%s",
  "version": "1.0.0",
  "description": "A sensorpanel theme",
  "type": "module",
  "width": 480,
  "height": 320,
  "scripts": {
    "dev": "vite",
    "build": "vite build",
    "preview": "vite preview"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0"
  },
  "devDependencies": {
    "@vitejs/plugin-react": "^4.2.1",
    "vite": "^5.0.0"
  }
}
`, name)
}

func viteConfig() string {
	return `import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  base: './',
  build: {
    outDir: 'dist',
    assetsDir: 'assets',
    // Inline everything for single-file output
    assetsInlineLimit: 100000,
    cssCodeSplit: false,
    rollupOptions: {
      output: {
        manualChunks: undefined,
        inlineDynamicImports: true,
      },
    },
  },
  server: {
    port: 3000,
  },
})
`
}

func indexHTML(name string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
  <head>
    <meta charset="UTF-8" />
    <meta name="viewport" content="width=480, height=320, initial-scale=1.0" />
    <title>%s - Sensor Panel</title>
    <style>
      * { margin: 0; padding: 0; box-sizing: border-box; }
      html, body, #root { width: 480px; height: 320px; overflow: hidden; }
    </style>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.jsx"></script>
  </body>
</html>
`, name)
}

func mainJSX() string {
	return `import React from 'react'
import ReactDOM from 'react-dom/client'
import App from './App'
import './App.css'

ReactDOM.createRoot(document.getElementById('root')).render(
  <React.StrictMode>
    <App />
  </React.StrictMode>,
)
`
}

func appJSX() string {
	return `import { useSensorData } from './hooks/useSensorData'

function App() {
  const data = useSensorData()

  const formatTemp = (temp) => temp != null ? ` + "`${temp.toFixed(0)}°C`" + ` : '--°C'
  const formatPercent = (pct) => pct != null ? ` + "`${pct.toFixed(0)}%`" + ` : '--%'
  const formatBytes = (bytes) => {
    if (bytes == null) return '--'
    if (bytes < 1024) return ` + "`${bytes} B/s`" + `
    if (bytes < 1024 * 1024) return ` + "`${(bytes / 1024).toFixed(1)} KB/s`" + `
    return ` + "`${(bytes / 1024 / 1024).toFixed(1)} MB/s`" + `
  }

  return (
    <div className="dashboard">
      {/* CPU Section */}
      <section className="section cpu">
        <div className="section-header">
          <span className="label">CPU</span>
          <span className="temp">{formatTemp(data.cpu?.temperature)}</span>
          <span className="freq">{data.cpu?.frequency_mhz?.toFixed(0) || '--'} MHz</span>
        </div>
        <div className="bar-container">
          <div 
            className="bar cpu-bar" 
            style={{ width: ` + "`${data.cpu?.load_percent || 0}%`" + ` }}
          />
          <span className="bar-text">{formatPercent(data.cpu?.load_percent)}</span>
        </div>
      </section>

      {/* GPU Section */}
      <section className="section gpu">
        <div className="section-header">
          <span className="label">GPU</span>
          <span className="temp">{formatTemp(data.gpu?.temperature)}</span>
          <span className="info">
            {data.gpu?.memory_used_mb?.toFixed(0) || '--'}/
            {data.gpu?.memory_total_mb?.toFixed(0) || '--'} MB
          </span>
        </div>
        <div className="bar-container">
          <div 
            className="bar gpu-bar" 
            style={{ width: ` + "`${data.gpu?.load_percent || 0}%`" + ` }}
          />
          <span className="bar-text">{formatPercent(data.gpu?.load_percent)}</span>
        </div>
      </section>

      {/* RAM Section */}
      <section className="section ram">
        <div className="section-header">
          <span className="label">RAM</span>
          <span className="info">
            {(data.memory?.used_mb / 1024).toFixed(1) || '--'}/
            {(data.memory?.total_mb / 1024).toFixed(1) || '--'} GB
          </span>
        </div>
        <div className="bar-container">
          <div 
            className="bar ram-bar" 
            style={{ width: ` + "`${data.memory?.percent || 0}%`" + ` }}
          />
          <span className="bar-text">{formatPercent(data.memory?.percent)}</span>
        </div>
      </section>

      {/* Disk Section */}
      {data.disks?.map((disk, i) => (
        <section key={i} className="section disk">
          <div className="section-header">
            <span className="label">{disk.mount_point}</span>
            <span className="info">
              {disk.used_gb?.toFixed(0) || '--'}/{disk.total_gb?.toFixed(0) || '--'} GB
            </span>
          </div>
          <div className="bar-container">
            <div 
              className="bar disk-bar" 
              style={{ width: ` + "`${disk.percent || 0}%`" + ` }}
            />
            <span className="bar-text">{formatPercent(disk.percent)}</span>
          </div>
        </section>
      ))}

      {/* Network Section */}
      <section className="section network">
        <div className="section-header">
          <span className="label">NET</span>
        </div>
        {data.networks?.filter(n => n.rx_bytes_per_sec > 0 || n.tx_bytes_per_sec > 0).map((net, i) => (
          <div key={i} className="net-row">
            <span className="net-iface">{net.interface}</span>
            <span className="net-rx">↓{formatBytes(net.rx_bytes_per_sec)}</span>
            <span className="net-tx">↑{formatBytes(net.tx_bytes_per_sec)}</span>
          </div>
        ))}
      </section>
    </div>
  )
}

export default App
`
}

func appCSS() string {
	return `* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}

html, body, #root {
  width: 480px;
  height: 320px;
  overflow: hidden;
  font-family: 'Segoe UI', system-ui, -apple-system, sans-serif;
}

.dashboard {
  width: 480px;
  height: 320px;
  background: linear-gradient(135deg, #1a1a2e 0%, #16213e 100%);
  color: #ffffff;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.section {
  background: rgba(255, 255, 255, 0.05);
  border-radius: 4px;
  padding: 6px 8px;
}

.section-header {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 4px;
  font-size: 12px;
}

.label {
  font-weight: 600;
  color: #888899;
  min-width: 35px;
}

.temp {
  color: #00ff88;
  font-weight: 500;
}

.freq, .info {
  color: #888899;
  font-size: 11px;
}

.bar-container {
  position: relative;
  height: 20px;
  background: #333344;
  border-radius: 3px;
  overflow: hidden;
}

.bar {
  height: 100%;
  transition: width 0.3s ease;
}

.cpu-bar { background: linear-gradient(90deg, #00d4ff, #0088cc); }
.gpu-bar { background: linear-gradient(90deg, #00ff88, #00cc66); }
.ram-bar { background: linear-gradient(90deg, #ff8800, #cc6600); }
.disk-bar { background: linear-gradient(90deg, #ff0088, #cc0066); }

.bar-text {
  position: absolute;
  top: 50%;
  left: 8px;
  transform: translateY(-50%);
  font-size: 11px;
  font-weight: 600;
  text-shadow: 0 1px 2px rgba(0,0,0,0.5);
}

.net-row {
  display: flex;
  gap: 10px;
  font-size: 11px;
  padding: 2px 0;
}

.net-iface {
  color: #8800ff;
  min-width: 60px;
}

.net-rx { color: #00ff88; }
.net-tx { color: #ff8800; }
`
}

func useSensorDataHook() string {
	return `import { useState, useEffect } from 'react'

// Mock data for development (shown until WebSocket connects)
const mockData = {
  cpu: {
    load_percent: 45,
    temperature: 52,
    frequency_mhz: 3600,
  },
  gpu: {
    available: true,
    load_percent: 30,
    temperature: 48,
    memory_used_mb: 2048,
    memory_total_mb: 8192,
    power_watts: 85,
  },
  memory: {
    total_mb: 32768,
    used_mb: 16384,
    percent: 50,
  },
  disks: [
    { mount_point: '/', total_gb: 500, used_gb: 250, percent: 50 },
  ],
  networks: [
    { interface: 'eth0', rx_bytes_per_sec: 1024000, tx_bytes_per_sec: 512000 },
  ],
}

export function useSensorData() {
  const [data, setData] = useState(mockData)

  useEffect(() => {
    // Get WebSocket port from query string (?ws=PORT) or use current page port
    const params = new URLSearchParams(window.location.search)
    const wsPort = params.get('ws') || window.location.port
    const wsHost = window.location.hostname || 'localhost'
    const wsUrl = ` + "`ws://${wsHost}:${wsPort}/ws`" + `
    
    let ws = null
    let reconnectTimer = null

    const connect = () => {
      try {
        ws = new WebSocket(wsUrl)
        
        ws.onopen = () => {
          console.log('WebSocket connected to', wsUrl)
        }
        
        ws.onmessage = (event) => {
          try {
            const newData = JSON.parse(event.data)
            setData(newData)
          } catch (e) {
            console.error('Failed to parse sensor data:', e)
          }
        }

        ws.onclose = () => {
          // Try to reconnect after 2 seconds
          reconnectTimer = setTimeout(connect, 2000)
        }

        ws.onerror = () => {
          ws.close()
        }
      } catch (e) {
        // WebSocket not available, use mock data
        console.log('WebSocket not available, using mock data')
      }
    }

    connect()

    // Also listen for postMessage (for preview mode)
    const handleMessage = (event) => {
      if (event.data && event.data.type === 'sensorData') {
        setData(event.data.data)
      }
    }
    window.addEventListener('message', handleMessage)

    return () => {
      if (ws) ws.close()
      if (reconnectTimer) clearTimeout(reconnectTimer)
      window.removeEventListener('message', handleMessage)
    }
  }, [])

  return data
}
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
  <title>%s - Sensor Panel</title>
  <style>
    * { margin: 0; padding: 0; box-sizing: border-box; }
    html, body { width: 480px; height: 320px; overflow: hidden; font-family: 'Segoe UI', system-ui, sans-serif; }
    
    .dashboard {
      width: 480px; height: 320px;
      background: linear-gradient(135deg, #1a1a2e 0%%, #16213e 100%%);
      color: #fff; padding: 8px;
      display: flex; flex-direction: column; gap: 6px;
    }
    .section { background: rgba(255,255,255,0.05); border-radius: 4px; padding: 6px 8px; }
    .section-header { display: flex; align-items: center; gap: 12px; margin-bottom: 4px; font-size: 12px; }
    .label { font-weight: 600; color: #888899; min-width: 35px; }
    .temp { color: #00ff88; font-weight: 500; }
    .info { color: #888899; font-size: 11px; }
    .bar-container { position: relative; height: 20px; background: #333344; border-radius: 3px; overflow: hidden; }
    .bar { height: 100%%; transition: width 0.3s ease; }
    .cpu-bar { background: linear-gradient(90deg, #00d4ff, #0088cc); }
    .gpu-bar { background: linear-gradient(90deg, #00ff88, #00cc66); }
    .ram-bar { background: linear-gradient(90deg, #ff8800, #cc6600); }
    .disk-bar { background: linear-gradient(90deg, #ff0088, #cc0066); }
    .bar-text { position: absolute; top: 50%%; left: 8px; transform: translateY(-50%%); font-size: 11px; font-weight: 600; text-shadow: 0 1px 2px rgba(0,0,0,0.5); }
    .net-row { display: flex; gap: 10px; font-size: 11px; padding: 2px 0; }
    .net-iface { color: #8800ff; min-width: 60px; }
    .net-rx { color: #00ff88; }
    .net-tx { color: #ff8800; }
  </style>
</head>
<body>
  <div class="dashboard" id="app">
    <section class="section">
      <div class="section-header">
        <span class="label">CPU</span>
        <span class="temp" id="cpu-temp">--°C</span>
        <span class="info" id="cpu-freq">-- MHz</span>
      </div>
      <div class="bar-container">
        <div class="bar cpu-bar" id="cpu-bar" style="width: 0%%"></div>
        <span class="bar-text" id="cpu-pct">--%%</span>
      </div>
    </section>

    <section class="section">
      <div class="section-header">
        <span class="label">GPU</span>
        <span class="temp" id="gpu-temp">--°C</span>
        <span class="info" id="gpu-mem">--/-- MB</span>
      </div>
      <div class="bar-container">
        <div class="bar gpu-bar" id="gpu-bar" style="width: 0%%"></div>
        <span class="bar-text" id="gpu-pct">--%%</span>
      </div>
    </section>

    <section class="section">
      <div class="section-header">
        <span class="label">RAM</span>
        <span class="info" id="ram-info">--/-- GB</span>
      </div>
      <div class="bar-container">
        <div class="bar ram-bar" id="ram-bar" style="width: 0%%"></div>
        <span class="bar-text" id="ram-pct">--%%</span>
      </div>
    </section>

    <section class="section">
      <div class="section-header">
        <span class="label" id="disk-label">/</span>
        <span class="info" id="disk-info">--/-- GB</span>
      </div>
      <div class="bar-container">
        <div class="bar disk-bar" id="disk-bar" style="width: 0%%"></div>
        <span class="bar-text" id="disk-pct">--%%</span>
      </div>
    </section>

    <section class="section">
      <div class="section-header">
        <span class="label">NET</span>
      </div>
      <div id="net-container"></div>
    </section>
  </div>

  <script>
    const $ = (id) => document.getElementById(id);
    const fmt = (v, suffix='') => v != null ? v.toFixed(0) + suffix : '--' + suffix;
    const fmtBytes = (b) => {
      if (b == null) return '--';
      if (b < 1024) return b + ' B/s';
      if (b < 1024*1024) return (b/1024).toFixed(1) + ' KB/s';
      return (b/1024/1024).toFixed(1) + ' MB/s';
    };

    function update(d) {
      if (d.cpu) {
        $('cpu-temp').textContent = fmt(d.cpu.temperature, '°C');
        $('cpu-freq').textContent = fmt(d.cpu.frequency_mhz, ' MHz');
        $('cpu-bar').style.width = (d.cpu.load_percent || 0) + '%%';
        $('cpu-pct').textContent = fmt(d.cpu.load_percent, '%%');
      }
      if (d.gpu) {
        $('gpu-temp').textContent = fmt(d.gpu.temperature, '°C');
        $('gpu-mem').textContent = fmt(d.gpu.memory_used_mb) + '/' + fmt(d.gpu.memory_total_mb) + ' MB';
        $('gpu-bar').style.width = (d.gpu.load_percent || 0) + '%%';
        $('gpu-pct').textContent = fmt(d.gpu.load_percent, '%%');
      }
      if (d.memory) {
        $('ram-info').textContent = (d.memory.used_mb/1024).toFixed(1) + '/' + (d.memory.total_mb/1024).toFixed(1) + ' GB';
        $('ram-bar').style.width = (d.memory.percent || 0) + '%%';
        $('ram-pct').textContent = fmt(d.memory.percent, '%%');
      }
      if (d.disks && d.disks.length > 0) {
        const disk = d.disks[0];
        $('disk-label').textContent = disk.mount_point || '/';
        $('disk-info').textContent = fmt(disk.used_gb) + '/' + fmt(disk.total_gb) + ' GB';
        $('disk-bar').style.width = (disk.percent || 0) + '%%';
        $('disk-pct').textContent = fmt(disk.percent, '%%');
      }
      if (d.networks) {
        const container = $('net-container');
        container.innerHTML = d.networks
          .filter(n => n.rx_bytes_per_sec > 0 || n.tx_bytes_per_sec > 0)
          .map(n => `+"`<div class=\"net-row\"><span class=\"net-iface\">${n.interface}</span><span class=\"net-rx\">↓${fmtBytes(n.rx_bytes_per_sec)}</span><span class=\"net-tx\">↑${fmtBytes(n.tx_bytes_per_sec)}</span></div>`"+`)
          .join('');
      }
    }

    // Connect to WebSocket
    // Use ?ws=PORT query param to specify custom WebSocket port (for dev mode)
    const params = new URLSearchParams(location.search);
    const wsPort = params.get('ws') || location.port;
    const wsUrl = `+"`ws://${location.hostname}:${wsPort}/ws`"+`;
    let ws, reconnect;
    function connect() {
      try {
        ws = new WebSocket(wsUrl);
        ws.onmessage = (e) => { try { update(JSON.parse(e.data)); } catch(err) {} };
        ws.onclose = () => { reconnect = setTimeout(connect, 2000); };
        ws.onerror = () => ws.close();
      } catch(e) {}
    }
    connect();

    // Also accept postMessage for preview
    window.addEventListener('message', (e) => {
      if (e.data && e.data.type === 'sensorData') update(e.data.data);
    });
  </script>
</body>
</html>
`, name)
}
