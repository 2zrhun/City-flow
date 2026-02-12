// CityFlow API client — fetch wrapper with JWT

const API_BASE = '';

export function getToken() {
    return localStorage.getItem('cityflow_token');
}

export function setToken(token) {
    localStorage.setItem('cityflow_token', token);
}

export function clearToken() {
    localStorage.removeItem('cityflow_token');
}

export function isAuthenticated() {
    return !!getToken();
}

async function request(method, path, body = null) {
    const headers = { 'Content-Type': 'application/json' };
    const token = getToken();
    if (token) headers['Authorization'] = `Bearer ${token}`;

    const opts = { method, headers };
    if (body) opts.body = JSON.stringify(body);

    const res = await fetch(`${API_BASE}${path}`, opts);

    if (res.status === 401) {
        clearToken();
        window.dispatchEvent(new CustomEvent('auth:expired'));
        throw new Error('Session expired');
    }

    if (!res.ok) {
        const err = await res.json().catch(() => ({ error: res.statusText }));
        throw new Error(err.error || 'Request failed');
    }

    return res.json();
}

// ── Auth ──

export async function login(email, password) {
    const data = await request('POST', '/api/auth/login', { email, password });
    setToken(data.token);
    return data;
}

export async function register(email, password) {
    const data = await request('POST', '/api/auth/register', { email, password });
    setToken(data.token);
    return data;
}

export async function logout() {
    try { await request('POST', '/api/auth/logout'); }
    finally { clearToken(); }
}

// ── Data ──

export async function getTrafficLive(options = {}) {
    const p = new URLSearchParams();
    if (options.limit) p.set('limit', options.limit);
    if (options.before) p.set('before', options.before);
    if (options.road_id) p.set('road_id', options.road_id);
    const qs = p.toString();
    return request('GET', `/api/traffic/live${qs ? '?' + qs : ''}`);
}

export async function getPredictions(options = {}) {
    const p = new URLSearchParams();
    p.set('horizon', options.horizon || '30');
    if (options.limit) p.set('limit', options.limit);
    if (options.before) p.set('before', options.before);
    if (options.road_id) p.set('road_id', options.road_id);
    return request('GET', `/api/predictions?${p.toString()}`);
}

export async function getRoads() {
    return request('GET', '/api/roads');
}

export async function getReroutes(options = {}) {
    const p = new URLSearchParams();
    if (options.limit) p.set('limit', options.limit);
    if (options.before) p.set('before', options.before);
    if (options.route_id) p.set('route_id', options.route_id);
    const qs = p.toString();
    return request('GET', `/api/reroutes/recommended${qs ? '?' + qs : ''}`);
}
