// CityFlow Dashboard — map, cards, predictions, reroutes, charts

import { getTrafficLive, getPredictions, getReroutes, logout } from './api.js';
import { TrafficWebSocket } from './websocket.js';

const ROADS = [
    'RING-NORTH-12', 'RING-SOUTH-09', 'CITY-CENTER-01',
    'AIRPORT-AXIS-03', 'UNIVERSITY-LOOP-07'
];

const ROAD_LABELS = {
    'RING-NORTH-12': 'Ring North',
    'RING-SOUTH-09': 'Ring South',
    'CITY-CENTER-01': 'City Center',
    'AIRPORT-AXIS-03': 'Airport Axis',
    'UNIVERSITY-LOOP-07': 'Univ. Loop'
};

const ROAD_COLORS = ['#3b82f6', '#8b5cf6', '#ec4899', '#f97316', '#06b6d4'];

const ROAD_GEO = {
    'RING-NORTH-12':      { x: 100, y: 50,  w: 600, h: 32, lx: 400, ly: 70 },
    'RING-SOUTH-09':      { x: 100, y: 418, w: 600, h: 32, lx: 400, ly: 438 },
    'CITY-CENTER-01':     { x: 270, y: 170, w: 260, h: 160, lx: 400, ly: 255 },
    'AIRPORT-AXIS-03':    { x: 95,  y: 50,  w: 32,  h: 400, lx: 60,  ly: 255 },
    'UNIVERSITY-LOOP-07': { x: 673, y: 50,  w: 32,  h: 400, lx: 740, ly: 255 },
};

// State
const liveState = {};
const historyState = {};
let speedChart = null;
let congestionChart = null;
let ws = null;
let pollTimers = [];

// ── Congestion helpers ──

function getCongestionLevel(speed, occupancy) {
    if (speed >= 60 && occupancy < 0.4) return 'free';
    if (speed < 30 || occupancy > 0.7) return 'congested';
    return 'moderate';
}

function getCongestionColor(level) {
    return { free: '#4ade80', moderate: '#fbbf24', congested: '#ef4444' }[level] || '#6b7280';
}

function getScoreLevel(score) {
    if (score < 0.35) return 'low';
    if (score < 0.65) return 'medium';
    return 'high';
}

// ── SVG Map ──

function buildMapSVG() {
    let roads = '';
    for (const [id, g] of Object.entries(ROAD_GEO)) {
        const isCenter = id === 'CITY-CENTER-01';
        const rx = isCenter ? 10 : 5;
        roads += `
            <rect id="road-${id}" class="road-segment" x="${g.x}" y="${g.y}"
                  width="${g.w}" height="${g.h}" rx="${rx}" fill="#6b7280"/>
            <text class="road-label" x="${g.lx}" y="${g.ly}"
                  text-anchor="middle" dominant-baseline="middle">${ROAD_LABELS[id]}</text>
            <text class="road-value" id="val-${id}" x="${g.lx}" y="${g.ly + 14}"
                  text-anchor="middle" dominant-baseline="middle">--</text>`;
    }

    // Connecting streets (thin lines from ring roads to city center)
    const conns = `
        <line x1="270" y1="250" x2="127" y2="250" stroke="#334155" stroke-width="2" stroke-dasharray="4"/>
        <line x1="530" y1="250" x2="673" y2="250" stroke="#334155" stroke-width="2" stroke-dasharray="4"/>
        <line x1="400" y1="170" x2="400" y2="82"  stroke="#334155" stroke-width="2" stroke-dasharray="4"/>
        <line x1="400" y1="330" x2="400" y2="418" stroke="#334155" stroke-width="2" stroke-dasharray="4"/>`;

    // Intersection circles
    const ints = `
        <circle cx="111" cy="66"  r="6" fill="#334155"/>
        <circle cx="689" cy="66"  r="6" fill="#334155"/>
        <circle cx="111" cy="434" r="6" fill="#334155"/>
        <circle cx="689" cy="434" r="6" fill="#334155"/>`;

    return `<svg viewBox="0 0 800 500" class="traffic-map">
        <rect width="800" height="500" fill="#0f0f23" rx="8"/>
        ${conns}${ints}${roads}
    </svg>`;
}

function updateMapRoad(roadId) {
    const d = liveState[roadId];
    if (!d) return;
    const level = getCongestionLevel(d.speed_kmh, d.occupancy);
    const color = getCongestionColor(level);
    const el = document.getElementById(`road-${roadId}`);
    if (el) el.setAttribute('fill', color);
    const valEl = document.getElementById(`val-${roadId}`);
    if (valEl) valEl.textContent = `${d.speed_kmh.toFixed(0)} km/h`;
}

// ── Metric Cards ──

function buildMetricCards() {
    return ROADS.map(id => `
        <div class="metric-card" id="card-${id}">
            <div class="metric-card-title">${ROAD_LABELS[id]}</div>
            <div class="metric-row"><span class="metric-label">Speed</span><span class="metric-value" id="speed-${id}">--</span></div>
            <div class="metric-row"><span class="metric-label">Flow</span><span class="metric-value" id="flow-${id}">--</span></div>
            <div class="metric-row"><span class="metric-label">Occupancy</span><span class="metric-value" id="occ-${id}">--</span></div>
        </div>`).join('');
}

function updateMetricCard(roadId) {
    const d = liveState[roadId];
    if (!d) return;
    const level = getCongestionLevel(d.speed_kmh, d.occupancy);
    const card = document.getElementById(`card-${roadId}`);
    if (card) { card.className = `metric-card ${level}`; }
    const s = document.getElementById(`speed-${roadId}`);
    const f = document.getElementById(`flow-${roadId}`);
    const o = document.getElementById(`occ-${roadId}`);
    if (s) s.textContent = `${d.speed_kmh.toFixed(1)} km/h`;
    if (f) f.textContent = `${d.flow_rate.toFixed(0)} veh/h`;
    if (o) o.textContent = `${(d.occupancy * 100).toFixed(0)}%`;
}

// ── Predictions ──

function renderPredictions(predictions) {
    const el = document.getElementById('predictions-list');
    if (!el) return;

    // Keep latest per road
    const latest = {};
    for (const p of predictions) {
        if (!latest[p.road_id] || p.ts > latest[p.road_id].ts) latest[p.road_id] = p;
    }

    if (Object.keys(latest).length === 0) {
        el.innerHTML = '<div class="empty-state">No predictions available yet</div>';
        return;
    }

    el.innerHTML = ROADS.filter(id => latest[id]).map(id => {
        const p = latest[id];
        const pct = (p.congestion_score * 100).toFixed(0);
        const level = getScoreLevel(p.congestion_score);
        const conf = (p.confidence * 100).toFixed(0);
        return `
            <div class="prediction-item">
                <span class="prediction-road">${ROAD_LABELS[id]}</span>
                <div class="prediction-bar-wrap">
                    <div class="prediction-bar ${level}" style="width:${pct}%"></div>
                </div>
                <span class="prediction-score">${pct}%</span>
                <span class="prediction-confidence">${conf}% conf</span>
            </div>`;
    }).join('');
}

// ── Reroutes ──

function renderReroutes(reroutes) {
    const el = document.getElementById('reroutes-list');
    if (!el) return;

    if (!reroutes || reroutes.length === 0) {
        el.innerHTML = '<div class="empty-state">No reroute recommendations</div>';
        return;
    }

    el.innerHTML = reroutes.map(r => `
        <div class="reroute-item">
            <div class="reroute-route">
                ${ROAD_LABELS[r.route_id] || r.route_id}
                <span class="reroute-arrow">&rarr;</span>
                ${ROAD_LABELS[r.alt_route_id] || r.alt_route_id}
            </div>
            <div class="reroute-reason">${r.reason}</div>
            <div class="reroute-badges">
                <span class="badge badge-co2">-${r.estimated_co2_gain.toFixed(1)} kg CO2</span>
                <span class="badge badge-eta">-${r.eta_gain_min.toFixed(0)} min</span>
            </div>
        </div>`).join('');
}

// ── Charts ──

function initCharts() {
    const speedCtx = document.getElementById('speed-chart');
    const congCtx = document.getElementById('congestion-chart');
    if (!speedCtx || !congCtx) return;

    const commonOpts = {
        responsive: true,
        maintainAspectRatio: false,
        animation: { duration: 300 },
        scales: {
            x: {
                type: 'time',
                time: { unit: 'minute', tooltipFormat: 'HH:mm:ss' },
                ticks: { color: '#94a3b8', maxTicksLimit: 8 },
                grid: { color: '#1e293b' }
            },
            y: {
                ticks: { color: '#94a3b8' },
                grid: { color: '#1e293b' }
            }
        },
        plugins: {
            legend: { labels: { color: '#e2e8f0', boxWidth: 12, padding: 10, font: { size: 11 } } }
        }
    };

    speedChart = new Chart(speedCtx, {
        type: 'line',
        data: {
            datasets: ROADS.map((road, i) => ({
                label: ROAD_LABELS[road],
                data: [],
                borderColor: ROAD_COLORS[i],
                backgroundColor: ROAD_COLORS[i] + '20',
                tension: 0.3,
                pointRadius: 0,
                borderWidth: 2
            }))
        },
        options: {
            ...commonOpts,
            scales: {
                ...commonOpts.scales,
                y: { ...commonOpts.scales.y, min: 0, max: 120, title: { display: true, text: 'km/h', color: '#94a3b8' } }
            }
        }
    });

    congestionChart = new Chart(congCtx, {
        type: 'line',
        data: {
            datasets: ROADS.map((road, i) => ({
                label: ROAD_LABELS[road],
                data: [],
                borderColor: ROAD_COLORS[i],
                backgroundColor: ROAD_COLORS[i] + '20',
                tension: 0.3,
                pointRadius: 0,
                borderWidth: 2
            }))
        },
        options: {
            ...commonOpts,
            scales: {
                ...commonOpts.scales,
                y: { ...commonOpts.scales.y, min: 0, max: 1, title: { display: true, text: 'Score', color: '#94a3b8' } }
            }
        }
    });
}

function pushSpeedPoint(roadId, ts, speed) {
    if (!speedChart) return;
    const idx = ROADS.indexOf(roadId);
    if (idx === -1) return;
    const ds = speedChart.data.datasets[idx];
    ds.data.push({ x: new Date(ts), y: speed });
    if (ds.data.length > 100) ds.data.shift();
    speedChart.update('none');
}

function updateCongestionChart(predictions) {
    if (!congestionChart) return;
    const latest = {};
    for (const p of predictions) {
        if (!latest[p.road_id] || p.ts > latest[p.road_id].ts) latest[p.road_id] = p;
    }
    for (const [roadId, p] of Object.entries(latest)) {
        const idx = ROADS.indexOf(roadId);
        if (idx === -1) continue;
        const ds = congestionChart.data.datasets[idx];
        ds.data.push({ x: new Date(p.ts), y: p.congestion_score });
        if (ds.data.length > 60) ds.data.shift();
    }
    congestionChart.update('none');
}

// ── WebSocket handler ──

function handleTrafficUpdate(data) {
    const roadId = data.road_id;
    if (!ROADS.includes(roadId)) return;

    liveState[roadId] = {
        speed_kmh: data.speed_kmh,
        flow_rate: data.flow_rate,
        occupancy: data.occupancy,
        ts: data.ts,
        sensor_id: data.sensor_id
    };

    updateMapRoad(roadId);
    updateMetricCard(roadId);
    pushSpeedPoint(roadId, data.ts, data.speed_kmh);
}

// ── Data fetching ──

async function loadInitialData() {
    try {
        const traffic = await getTrafficLive({ limit: 200 });
        // Process from oldest to newest
        const sorted = (traffic.data || []).slice().reverse();
        for (const d of sorted) {
            liveState[d.road_id] = d;
            pushSpeedPoint(d.road_id, d.ts, d.speed_kmh);
        }
        ROADS.forEach(id => { updateMapRoad(id); updateMetricCard(id); });
    } catch (err) { console.error('[dashboard] load traffic:', err); }

    await refreshPredictions();
    await refreshReroutes();
}

async function refreshPredictions() {
    try {
        const res = await getPredictions({ limit: 50 });
        renderPredictions(res.data || []);
        updateCongestionChart(res.data || []);
    } catch (err) { console.error('[dashboard] load predictions:', err); }
}

async function refreshReroutes() {
    try {
        const res = await getReroutes({ limit: 20 });
        renderReroutes(res.data || []);
    } catch (err) { console.error('[dashboard] load reroutes:', err); }
}

// ── Render ──

export function renderDashboard(container, onLogout) {
    container.innerHTML = `
        <header class="dashboard-header">
            <h1>CityFlow</h1>
            <div class="header-right">
                <span id="ws-status" class="status-badge connecting">Connecting...</span>
                <button id="btn-logout">Logout</button>
            </div>
        </header>
        <main class="dashboard-grid">
            <section class="panel map-panel">
                <h2>Live Traffic Map</h2>
                <div id="traffic-map">${buildMapSVG()}</div>
            </section>
            <section class="panel metrics-panel">
                <h2>Real-Time Metrics</h2>
                <div id="metric-cards" class="cards-grid">${buildMetricCards()}</div>
            </section>
            <section class="panel">
                <h2>Congestion Predictions (T+30min)</h2>
                <div id="predictions-list"><div class="empty-state">Loading...</div></div>
            </section>
            <section class="panel">
                <h2>Reroute Recommendations</h2>
                <div id="reroutes-list"><div class="empty-state">Loading...</div></div>
            </section>
            <section class="panel chart-panel">
                <h2>Speed Trends</h2>
                <div style="position:relative;height:250px"><canvas id="speed-chart"></canvas></div>
            </section>
            <section class="panel chart-panel">
                <h2>Congestion Forecast</h2>
                <div style="position:relative;height:250px"><canvas id="congestion-chart"></canvas></div>
            </section>
        </main>`;

    // Logout
    document.getElementById('btn-logout').addEventListener('click', async () => {
        try { await logout(); } catch {}
        onLogout();
    });

    // WS status
    const statusEl = document.getElementById('ws-status');
    function setStatus(s) {
        if (!statusEl) return;
        statusEl.className = `status-badge ${s}`;
        statusEl.textContent = s === 'connected' ? 'Live' : s === 'connecting' ? 'Connecting...' : 'Disconnected';
    }

    // Init charts
    initCharts();

    // Start WebSocket
    ws = new TrafficWebSocket(handleTrafficUpdate, setStatus);
    ws.connect();

    // Load initial data
    loadInitialData();

    // Poll predictions + reroutes every 30s
    pollTimers.push(setInterval(refreshPredictions, 30000));
    pollTimers.push(setInterval(refreshReroutes, 30000));
}

export function destroyDashboard() {
    if (ws) { ws.disconnect(); ws = null; }
    pollTimers.forEach(t => clearInterval(t));
    pollTimers = [];
    if (speedChart) { speedChart.destroy(); speedChart = null; }
    if (congestionChart) { congestionChart.destroy(); congestionChart = null; }
    Object.keys(liveState).forEach(k => delete liveState[k]);
    Object.keys(historyState).forEach(k => delete historyState[k]);
}
