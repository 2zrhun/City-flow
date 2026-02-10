// CityFlow WebSocket client — auto-reconnect + double-parse

import { getToken } from './api.js';

export class TrafficWebSocket {
    constructor(onMessage, onStatusChange) {
        this.onMessage = onMessage;
        this.onStatusChange = onStatusChange || (() => {});
        this.ws = null;
        this.reconnectTimer = null;
        this.reconnectDelay = 1000;
        this.maxReconnectDelay = 30000;
        this.intentionalClose = false;
    }

    connect() {
        const token = getToken();
        if (!token) return;

        this.intentionalClose = false;
        const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = `${protocol}//${location.host}/ws/live?token=${encodeURIComponent(token)}`;

        this.ws = new WebSocket(wsUrl);
        this.onStatusChange('connecting');

        this.ws.onopen = () => {
            this.reconnectDelay = 1000;
            this.onStatusChange('connected');
        };

        this.ws.onmessage = (event) => {
            try {
                const envelope = JSON.parse(event.data);
                if (envelope.type === 'traffic_update') {
                    // Backend sends data as a JSON *string* (msg.Payload from Redis)
                    // so we need a second JSON.parse
                    const trafficData = typeof envelope.data === 'string'
                        ? JSON.parse(envelope.data)
                        : envelope.data;
                    this.onMessage(trafficData);
                }
            } catch (err) {
                console.error('[ws] parse error:', err);
            }
        };

        this.ws.onclose = () => {
            this.onStatusChange('disconnected');
            if (!this.intentionalClose) this.scheduleReconnect();
        };

        this.ws.onerror = () => {};
    }

    scheduleReconnect() {
        clearTimeout(this.reconnectTimer);
        this.reconnectTimer = setTimeout(() => {
            this.reconnectDelay = Math.min(this.reconnectDelay * 2, this.maxReconnectDelay);
            this.connect();
        }, this.reconnectDelay);
    }

    disconnect() {
        this.intentionalClose = true;
        clearTimeout(this.reconnectTimer);
        if (this.ws) { this.ws.close(); this.ws = null; }
    }
}
