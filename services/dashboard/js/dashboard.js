// CityFlow Dashboard — Leaflet map + navigation (Waze/Maps style)

import { getTrafficLive, getPredictions, logout } from './api.js';
import { TrafficWebSocket } from './websocket.js';

const ROADS = [
    'RING-NORTH-12', 'RING-SOUTH-09', 'CITY-CENTER-01',
    'AIRPORT-AXIS-03', 'UNIVERSITY-LOOP-07'
];

const ROAD_LABELS = {
    'RING-NORTH-12': 'Ring North 12',
    'RING-SOUTH-09': 'Ring South 09',
    'CITY-CENTER-01': 'City Center 01',
    'AIRPORT-AXIS-03': 'Airport Axis 03',
    'UNIVERSITY-LOOP-07': 'University Loop 07'
};

// Road coordinates — used for traffic proximity checks on routes
const ROAD_PATHS = {
    'RING-NORTH-12': [
        [48.8720, 2.3200], [48.8730, 2.3280], [48.8735, 2.3380],
        [48.8730, 2.3480], [48.8720, 2.3550]
    ],
    'RING-SOUTH-09': [
        [48.8530, 2.3200], [48.8520, 2.3280], [48.8515, 2.3380],
        [48.8520, 2.3480], [48.8530, 2.3550]
    ],
    'CITY-CENTER-01': [
        [48.8650, 2.3320], [48.8640, 2.3350], [48.8625, 2.3380],
        [48.8610, 2.3410], [48.8600, 2.3440]
    ],
    'AIRPORT-AXIS-03': [
        [48.8720, 2.3200], [48.8680, 2.3180], [48.8630, 2.3170],
        [48.8580, 2.3185], [48.8530, 2.3200]
    ],
    'UNIVERSITY-LOOP-07': [
        [48.8720, 2.3550], [48.8680, 2.3570], [48.8630, 2.3580],
        [48.8580, 2.3565], [48.8530, 2.3550]
    ]
};

// State
const liveState = {};
let leafletMap = null;
let ws = null;
let predictionsTimer = null;

// Navigation state
let routingControl = null;
let navStartMarker = null;
let navEndMarker = null;
let navStartLatLng = null;
let navEndLatLng = null;
let navClickTarget = 'from';
let navGeocoder = null;
let navGeocoderTimeout = null;
let activeRouteIndex = 0;
let navRoutesData = [];

// ── Congestion helpers ──

function getCongestionLevel(speed, occupancy) {
    if (speed >= 60 && occupancy < 0.4) return 'free';
    if (speed < 30 || occupancy > 0.7) return 'congested';
    return 'moderate';
}

// ── CO2 Estimation ──

function estimateCO2(route) {
    const distKm = route.summary.totalDistance / 1000;
    const timeH = route.summary.totalTime / 3600;
    const avgSpeed = timeH > 0 ? distKm / timeH : 50;
    let factor; // g CO2/km (speed-based emission curve)
    if (avgSpeed < 15) factor = 280;       // heavy stop-and-go
    else if (avgSpeed < 30) factor = 230;  // congested
    else if (avgSpeed < 50) factor = 180;  // moderate urban
    else if (avgSpeed < 80) factor = 150;  // free flow (optimal)
    else factor = 170;                     // highway (less efficient)
    return (factor * distKm) / 1000;       // kg CO2
}

// ── Leaflet Map ──

function initMap() {
    leafletMap = L.map('map', {
        center: [48.8625, 2.3390],
        zoom: 15,
        zoomControl: true,
        attributionControl: false
    });

    L.tileLayer('https://{s}.basemaps.cartocdn.com/dark_all/{z}/{x}/{y}{r}.png', {
        maxZoom: 19,
        subdomains: 'abcd'
    }).addTo(leafletMap);

    L.control.attribution({ position: 'bottomright', prefix: false })
        .addAttribution('&copy; <a href="https://www.openstreetmap.org/copyright">OSM</a> &copy; <a href="https://carto.com/">CARTO</a>')
        .addTo(leafletMap);
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
    if (routingControl && navRoutesData.length > 0) {
        checkTrafficOnRoute(navRoutesData[activeRouteIndex]);
    }
}

// ── Data fetching ──

async function loadInitialData() {
    try {
        const traffic = await getTrafficLive({ limit: 200 });
        const sorted = (traffic.data || []).slice().reverse();
        for (const d of sorted) {
            liveState[d.road_id] = d;
        }
    } catch (err) { console.error('[dashboard] load traffic:', err); }
}

// ── Predictions ──

function getCurrentCongestion(roadId) {
    const d = liveState[roadId];
    if (!d) return null;
    const speed = d.speed_kmh || 0;
    const occ = d.occupancy || 0;
    const speedScore = 1.0 - Math.min(speed / 90, 1);
    const flowScore = Math.min((d.flow_rate || 0) / 120, 1);
    return 0.4 * speedScore + 0.4 * occ + 0.2 * flowScore;
}

function getCongestionColor(score) {
    if (score < 0.3) return 'var(--accent-green)';
    if (score < 0.6) return 'var(--accent-yellow)';
    return 'var(--accent-red)';
}

function getCongestionLabel(score) {
    if (score < 0.3) return 'Free';
    if (score < 0.6) return 'Moderate';
    return 'Congested';
}

function getTrendInfo(current, predicted) {
    if (current === null) return { arrow: '&mdash;', cls: 'stable', label: 'N/A' };
    const delta = predicted - current;
    if (delta > 0.08) return { arrow: '&#9650;', cls: 'up', label: `+${(delta * 100).toFixed(0)}%` };
    if (delta < -0.08) return { arrow: '&#9660;', cls: 'down', label: `${(delta * 100).toFixed(0)}%` };
    return { arrow: '&#9654;', cls: 'stable', label: 'Stable' };
}

async function loadPredictions() {
    const list = document.getElementById('predictions-list');
    const tsEl = document.getElementById('predictions-updated');
    if (!list) return;

    try {
        const res = await getPredictions({ horizon: 30, limit: 50 });
        const rows = res.data || [];
        if (rows.length === 0) {
            list.innerHTML = '<div class="empty-state">No predictions available yet</div>';
            return;
        }

        // Latest prediction per road
        const latest = {};
        for (const r of rows) {
            if (!latest[r.road_id] || r.ts > latest[r.road_id].ts) {
                latest[r.road_id] = r;
            }
        }

        list.innerHTML = ROADS.map(roadId => {
            const pred = latest[roadId];
            if (!pred) return '';
            const score = pred.congestion_score;
            const conf = pred.confidence != null ? pred.confidence : 0;
            const color = getCongestionColor(score);
            const label = getCongestionLabel(score);
            const current = getCurrentCongestion(roadId);
            const trend = getTrendInfo(current, score);
            const pct = Math.round(score * 100);

            return `<div class="prediction-card">
                <div class="prediction-header">
                    <span class="prediction-road">${ROAD_LABELS[roadId]}</span>
                    <span class="prediction-trend ${trend.cls}">${trend.arrow} ${trend.label}</span>
                </div>
                <div class="prediction-bar-wrap">
                    <div class="prediction-bar" style="width:${pct}%;background:${color}"></div>
                </div>
                <div class="prediction-meta">
                    <span class="prediction-label" style="color:${color}">${label} (${pct}%)</span>
                    <span class="prediction-confidence">${(conf * 100).toFixed(0)}% conf.</span>
                </div>
            </div>`;
        }).join('');

        if (tsEl && rows.length > 0) {
            const t = new Date(rows[0].ts);
            tsEl.textContent = `Updated ${t.toLocaleTimeString()}`;
        }
    } catch (err) {
        console.error('[dashboard] load predictions:', err);
        list.innerHTML = '<div class="empty-state">Could not load predictions</div>';
    }
}

// ── Navigation: Geocoder ──

function initGeocoder() {
    navGeocoder = L.Control.Geocoder.nominatim({
        geocodingQueryParams: {
            viewbox: '2.28,48.89,2.40,48.83',
            bounded: 0
        }
    });
}

function setupGeocoderAutocomplete(inputId, onSelect) {
    const input = document.getElementById(inputId);
    if (!input) return;
    let suggestionsEl = null;

    function removeSuggestions() {
        if (suggestionsEl && suggestionsEl.parentElement) {
            suggestionsEl.parentElement.removeChild(suggestionsEl);
        }
        suggestionsEl = null;
    }

    input.addEventListener('input', () => {
        clearTimeout(navGeocoderTimeout);
        const query = input.value.trim();
        if (query.length < 3) { removeSuggestions(); return; }

        navGeocoderTimeout = setTimeout(() => {
            navGeocoder.geocode(query, (results) => {
                removeSuggestions();
                if (!results || results.length === 0) return;
                suggestionsEl = document.createElement('div');
                suggestionsEl.className = 'geocoder-suggestions';
                input.parentElement.appendChild(suggestionsEl);

                results.slice(0, 5).forEach(result => {
                    const item = document.createElement('div');
                    item.className = 'geocoder-suggestion';
                    item.textContent = result.name;
                    item.addEventListener('click', () => {
                        input.value = result.name;
                        removeSuggestions();
                        onSelect(result.center);
                    });
                    suggestionsEl.appendChild(item);
                });
            });
        }, 400);
    });

    document.addEventListener('click', (e) => {
        if (!input.contains(e.target) && (!suggestionsEl || !suggestionsEl.contains(e.target))) {
            removeSuggestions();
        }
    });
}

// ── Navigation: Map Click & Markers ──

function setupMapClickForNav() {
    leafletMap.on('click', (e) => {
        if (navClickTarget === 'from') {
            setNavStart(e.latlng);
            navClickTarget = 'to';
        } else {
            setNavEnd(e.latlng);
            navClickTarget = 'from';
        }
        updateDirectionsButton();
    });
}

function setNavStart(latlng) {
    navStartLatLng = latlng;
    if (navStartMarker) {
        navStartMarker.setLatLng(latlng);
    } else {
        navStartMarker = L.marker(latlng, {
            icon: L.divIcon({ className: 'nav-marker-start', iconSize: [16, 16], iconAnchor: [8, 8] }),
            draggable: true
        }).addTo(leafletMap);
        navStartMarker.on('dragend', () => {
            navStartLatLng = navStartMarker.getLatLng();
            reverseGeocode(navStartLatLng, 'nav-from');
            updateDirectionsButton();
        });
    }
    reverseGeocode(latlng, 'nav-from');
}

function setNavEnd(latlng) {
    navEndLatLng = latlng;
    if (navEndMarker) {
        navEndMarker.setLatLng(latlng);
    } else {
        navEndMarker = L.marker(latlng, {
            icon: L.divIcon({ className: 'nav-marker-end', iconSize: [16, 16], iconAnchor: [8, 8] }),
            draggable: true
        }).addTo(leafletMap);
        navEndMarker.on('dragend', () => {
            navEndLatLng = navEndMarker.getLatLng();
            reverseGeocode(navEndLatLng, 'nav-to');
            updateDirectionsButton();
        });
    }
    reverseGeocode(latlng, 'nav-to');
}

function reverseGeocode(latlng, inputId) {
    const input = document.getElementById(inputId);
    if (!input) return;
    input.value = `${latlng.lat.toFixed(5)}, ${latlng.lng.toFixed(5)}`;
    navGeocoder.reverse(latlng, leafletMap.options.crs.scale(leafletMap.getZoom()), (results) => {
        if (results && results.length > 0) input.value = results[0].name;
    });
}

function updateDirectionsButton() {
    const btn = document.getElementById('nav-directions-btn');
    if (btn) btn.disabled = !(navStartLatLng && navEndLatLng);
}

// ── Navigation: Routing ──

function calculateRoute() {
    if (!navStartLatLng || !navEndLatLng) return;
    if (routingControl) { leafletMap.removeControl(routingControl); routingControl = null; }

    routingControl = L.Routing.control({
        waypoints: [
            L.latLng(navStartLatLng.lat, navStartLatLng.lng),
            L.latLng(navEndLatLng.lat, navEndLatLng.lng)
        ],
        router: L.Routing.osrmv1({
            serviceUrl: 'https://router.project-osrm.org/route/v1',
            profile: 'driving',
            timeout: 10000
        }),
        routeWhileDragging: false,
        showAlternatives: true,
        altLineOptions: {
            styles: [{ color: '#6366f1', opacity: 0.4, weight: 6 }],
            extendToWaypoints: true,
            missingRouteTolerance: 5
        },
        lineOptions: {
            styles: [
                { color: '#3b82f6', opacity: 0.8, weight: 7 },
                { color: '#1d4ed8', opacity: 0.3, weight: 11 }
            ],
            extendToWaypoints: true,
            missingRouteTolerance: 5
        },
        createMarker: () => null,
        addWaypoints: false,
        fitSelectedRoutes: true,
        show: false
    }).addTo(leafletMap);

    routingControl.on('routesfound', (e) => {
        navRoutesData = e.routes;
        activeRouteIndex = 0;
        renderRouteSummary(e.routes);
        renderDirections(e.routes[0]);
        checkTrafficOnRoute(e.routes[0]);
    });

    routingControl.on('routeselected', (e) => {
        const idx = navRoutesData.indexOf(e.route);
        if (idx >= 0) activeRouteIndex = idx;
        renderDirections(e.route);
        checkTrafficOnRoute(e.route);
        const container = document.getElementById('nav-routes-list');
        if (container) {
            container.querySelectorAll('.route-option').forEach(opt =>
                opt.classList.toggle('selected', parseInt(opt.dataset.routeIndex) === activeRouteIndex)
            );
        }
    });

    routingControl.on('routingerror', () => {
        const summaryEl = document.getElementById('nav-routes-list');
        if (summaryEl) summaryEl.innerHTML = '<div class="empty-state">Could not find a route. Try different points.</div>';
        document.getElementById('nav-route-summary')?.classList.remove('hidden');
    });
}

// ── Navigation: Route Summary ──

function renderRouteSummary(routes) {
    const container = document.getElementById('nav-routes-list');
    const panel = document.getElementById('nav-route-summary');
    if (!container || !panel) return;
    panel.classList.remove('hidden');

    // Compute CO2 for all routes and find the greenest
    const co2Values = routes.map(r => estimateCO2(r));
    const minCO2 = Math.min(...co2Values);

    container.innerHTML = routes.map((route, i) => {
        const distKm = (route.summary.totalDistance / 1000).toFixed(1);
        const etaMin = Math.round(route.summary.totalTime / 60);
        const etaStr = etaMin >= 60 ? `${Math.floor(etaMin / 60)}h ${etaMin % 60}min` : `${etaMin} min`;
        const name = route.name || `Route ${i + 1}`;
        const co2Kg = co2Values[i];
        const isEco = co2Kg === minCO2 && routes.length > 1;
        return `<div class="route-option ${i === activeRouteIndex ? 'selected' : ''}" data-route-index="${i}">
            <div class="route-option-index">${i + 1}</div>
            <div class="route-option-details">
                <div class="route-option-name">${name}</div>
                <div class="route-option-meta">
                    <span><b>${distKm}</b> km</span>
                    <span><b>${etaStr}</b></span>
                </div>
                <div class="route-co2">
                    &#127807; <b>${co2Kg.toFixed(1)} kg</b> CO&#8322;${isEco ? '<span class="eco-badge">Eco</span>' : ''}
                </div>
            </div>
        </div>`;
    }).join('');

    container.querySelectorAll('.route-option').forEach(el => {
        el.addEventListener('click', () => {
            const idx = parseInt(el.dataset.routeIndex);
            activeRouteIndex = idx;
            container.querySelectorAll('.route-option').forEach(opt =>
                opt.classList.toggle('selected', parseInt(opt.dataset.routeIndex) === idx)
            );
            if (navRoutesData[idx]) {
                renderDirections(navRoutesData[idx]);
                checkTrafficOnRoute(navRoutesData[idx]);
            }
        });
    });
}

// ── Navigation: Turn-by-Turn ──

function renderDirections(route) {
    const panel = document.getElementById('nav-directions-panel');
    const list = document.getElementById('nav-directions-list');
    if (!panel || !list) return;
    panel.classList.remove('hidden');

    if (!route.instructions || route.instructions.length === 0) {
        list.innerHTML = '<div class="empty-state">No detailed directions available</div>';
        return;
    }

    list.innerHTML = route.instructions.map((instr, i) => {
        const distStr = instr.distance >= 1000
            ? `${(instr.distance / 1000).toFixed(1)} km`
            : `${Math.round(instr.distance)} m`;
        return `<div class="direction-step">
            <div class="direction-step-num">${i + 1}</div>
            <div class="direction-step-text">${instr.text}</div>
            <div class="direction-step-dist">${distStr}</div>
        </div>`;
    }).join('');
}

// ── Navigation: CityFlow Traffic Integration ──

function checkTrafficOnRoute(route) {
    const warningsPanel = document.getElementById('nav-traffic-warnings');
    const warningsList = document.getElementById('nav-warnings-list');
    if (!warningsPanel || !warningsList) return;

    const routeCoords = route.coordinates;
    const warnings = [];

    for (const [roadId, path] of Object.entries(ROAD_PATHS)) {
        const roadData = liveState[roadId];
        if (!roadData) continue;
        const level = getCongestionLevel(roadData.speed_kmh, roadData.occupancy);
        if (level === 'free') continue;

        const isNearRoute = path.some(roadPoint =>
            routeCoords.some(routePoint =>
                leafletMap.distance(L.latLng(roadPoint[0], roadPoint[1]), routePoint) < 150
            )
        );

        if (isNearRoute) {
            warnings.push({
                roadId,
                label: ROAD_LABELS[roadId],
                level,
                speed: roadData.speed_kmh,
                occupancy: roadData.occupancy
            });
        }
    }

    if (warnings.length === 0) { warningsPanel.classList.add('hidden'); return; }

    warningsPanel.classList.remove('hidden');
    warningsList.innerHTML = warnings.map(w => `
        <div class="traffic-warning ${w.level}">
            <span class="traffic-warning-icon">&#9888;</span>
            <div class="traffic-warning-text">
                <span class="traffic-warning-road">${w.label}</span>
                &mdash; ${w.level === 'congested' ? 'Heavy traffic' : 'Moderate traffic'}
                (${w.speed.toFixed(0)} km/h, ${(w.occupancy * 100).toFixed(0)}% occupancy)
            </div>
        </div>
    `).join('');
}

// ── Navigation: Clear ──

function clearNavRoute() {
    if (routingControl) { leafletMap.removeControl(routingControl); routingControl = null; }
    if (navStartMarker) { leafletMap.removeLayer(navStartMarker); navStartMarker = null; }
    if (navEndMarker) { leafletMap.removeLayer(navEndMarker); navEndMarker = null; }
    navStartLatLng = null;
    navEndLatLng = null;
    navClickTarget = 'from';
    navRoutesData = [];
    activeRouteIndex = 0;

    const fromInput = document.getElementById('nav-from');
    const toInput = document.getElementById('nav-to');
    if (fromInput) fromInput.value = '';
    if (toInput) toInput.value = '';

    document.getElementById('nav-route-summary')?.classList.add('hidden');
    document.getElementById('nav-traffic-warnings')?.classList.add('hidden');
    document.getElementById('nav-directions-panel')?.classList.add('hidden');
    document.getElementById('nav-click-hint')?.classList.remove('hidden');
    updateDirectionsButton();
}

// ── Render ──

export function renderDashboard(container, onLogout) {
    container.innerHTML = `
        <div class="dashboard-wrap">
            <div id="map"></div>

            <div class="top-bar">
                <h1>CityFlow</h1>
                <div class="top-bar-right">
                    <span id="ws-status" class="status-badge connecting">Connecting...</span>
                    <button id="btn-logout">Logout</button>
                </div>
            </div>

            <div class="sidebar-left">
                <div class="panel nav-panel">
                    <h2><span class="icon">&#9656;</span> Get Directions</h2>
                    <div class="nav-inputs">
                        <div class="nav-input-row">
                            <span class="nav-dot nav-dot-start"></span>
                            <input type="text" id="nav-from" class="nav-input" placeholder="Starting point (click map or type)" autocomplete="off" />
                            <button class="nav-input-clear" id="nav-clear-from" title="Clear">&times;</button>
                        </div>
                        <div class="nav-input-connector"></div>
                        <div class="nav-input-row">
                            <span class="nav-dot nav-dot-end"></span>
                            <input type="text" id="nav-to" class="nav-input" placeholder="Destination (click map or type)" autocomplete="off" />
                            <button class="nav-input-clear" id="nav-clear-to" title="Clear">&times;</button>
                        </div>
                    </div>
                    <div class="nav-actions">
                        <button id="nav-directions-btn" class="btn-primary nav-btn" disabled>Get Directions</button>
                        <button id="nav-clear-btn" class="nav-btn-secondary">Clear</button>
                    </div>
                    <div id="nav-click-hint" class="nav-hint">Click on the map to set start and destination points</div>
                </div>

                <div id="nav-route-summary" class="panel hidden">
                    <h2><span class="icon">&#9737;</span> Route Options</h2>
                    <div id="nav-routes-list"></div>
                </div>

                <div id="nav-traffic-warnings" class="panel hidden">
                    <h2><span class="icon">&#9888;</span> Traffic Alerts on Route</h2>
                    <div id="nav-warnings-list"></div>
                </div>

                <div id="nav-directions-panel" class="panel hidden">
                    <h2><span class="icon">&#10132;</span> Directions</h2>
                    <div id="nav-directions-list" class="directions-list"></div>
                </div>
            </div>

            <div class="sidebar-right">
                <div class="panel predictions-panel">
                    <h2><span class="icon">&#9201;</span> Predictions T+30min</h2>
                    <div id="predictions-list" class="predictions-list">
                        <div class="empty-state">Loading predictions...</div>
                    </div>
                    <div class="predictions-footer">
                        <span id="predictions-updated" class="predictions-ts"></span>
                    </div>
                </div>
            </div>

        </div>`;

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

    // Init Leaflet map
    initMap();

    // Navigation setup
    initGeocoder();
    setupMapClickForNav();
    leafletMap.getContainer().style.cursor = 'crosshair';

    setupGeocoderAutocomplete('nav-from', (latlng) => {
        setNavStart(latlng);
        navClickTarget = 'to';
        updateDirectionsButton();
    });
    setupGeocoderAutocomplete('nav-to', (latlng) => {
        setNavEnd(latlng);
        navClickTarget = 'from';
        updateDirectionsButton();
    });

    document.getElementById('nav-directions-btn')?.addEventListener('click', () => {
        calculateRoute();
        document.getElementById('nav-click-hint')?.classList.add('hidden');
    });
    document.getElementById('nav-clear-btn')?.addEventListener('click', clearNavRoute);
    document.getElementById('nav-clear-from')?.addEventListener('click', () => {
        navStartLatLng = null;
        if (navStartMarker) { leafletMap.removeLayer(navStartMarker); navStartMarker = null; }
        document.getElementById('nav-from').value = '';
        navClickTarget = 'from';
        updateDirectionsButton();
    });
    document.getElementById('nav-clear-to')?.addEventListener('click', () => {
        navEndLatLng = null;
        if (navEndMarker) { leafletMap.removeLayer(navEndMarker); navEndMarker = null; }
        document.getElementById('nav-to').value = '';
        navClickTarget = 'to';
        updateDirectionsButton();
    });

    // Start WebSocket
    ws = new TrafficWebSocket(handleTrafficUpdate, setStatus);
    ws.connect();

    // Load initial data
    loadInitialData();

    // Load predictions and refresh every 60s
    loadPredictions();
    predictionsTimer = setInterval(loadPredictions, 60000);
}

export function destroyDashboard() {
    if (ws) { ws.disconnect(); ws = null; }
    if (predictionsTimer) { clearInterval(predictionsTimer); predictionsTimer = null; }
    // Navigation cleanup (before map removal)
    if (routingControl) { leafletMap.removeControl(routingControl); routingControl = null; }
    if (navStartMarker) { leafletMap.removeLayer(navStartMarker); navStartMarker = null; }
    if (navEndMarker) { leafletMap.removeLayer(navEndMarker); navEndMarker = null; }
    navStartLatLng = null;
    navEndLatLng = null;
    navGeocoder = null;
    navRoutesData = [];
    clearTimeout(navGeocoderTimeout);
    if (leafletMap) { leafletMap.remove(); leafletMap = null; }
    Object.keys(liveState).forEach(k => delete liveState[k]);
}
