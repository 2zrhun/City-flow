const mqtt = require("mqtt");

const brokerUrl = process.env.MQTT_URL || "mqtt://localhost:1883";
const publishIntervalMs = Number(process.env.PUBLISH_INTERVAL_MS || 2000);
const refreshIntervalMs = Number(process.env.REFRESH_INTERVAL_MS || 3600000); // 1h
const maxSensors = Number(process.env.MAX_SENSORS || 100);

// ── Paris Open Data: dynamic sensor discovery ──

const API_BASE = "https://opendata.paris.fr/api/explore/v2.1/catalog/datasets/comptages-routiers-permanents/records";

// Greenshields speed model
const FREE_FLOW_SPEED = 50;  // km/h (Paris urban)
const JAM_OCCUPANCY = 30;    // % (loop sensor jam threshold)

function estimateSpeed(k) {
  if (k == null || k <= 0) return FREE_FLOW_SPEED;
  const speed = FREE_FLOW_SPEED * (1 - k / JAM_OCCUPANCY);
  return Math.max(5, Math.min(90, speed));
}

// ── Data cache ──

const dataCache = {};  // roadId → [records]
const cursors = {};    // roadId → index
const roadMeta = {};   // roadId → { label, lat, lng }
let activeRoads = [];  // list of road IDs with data
let usingRealData = false;

// ── Fetch from Paris Open Data (all sensors) ──

async function fetchParisData() {
  const where = "q is not null and k is not null";
  const select = "iu_ac,libelle,q,k,etat_trafic,t_1h,geo_point_2d";
  const url = `${API_BASE}?where=${encodeURIComponent(where)}&select=${select}&order_by=t_1h+desc&limit=${maxSensors}`;

  try {
    const res = await fetch(url, {
      headers: { "User-Agent": "CityFlow-Simulator/2.0" }
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const json = await res.json();
    const records = json.results || [];

    if (records.length === 0) {
      console.warn("[simulator] Paris API returned 0 records, keeping previous cache");
      return;
    }

    // Clear caches
    for (const k of Object.keys(dataCache)) delete dataCache[k];
    for (const k of Object.keys(roadMeta)) delete roadMeta[k];

    // Group by sensor (iu_ac) — each sensor = one road
    for (const rec of records) {
      const sensorId = String(rec.iu_ac);
      const roadId = `PARIS-${sensorId}`;

      if (!dataCache[roadId]) {
        dataCache[roadId] = [];
      }

      // Store metadata (first occurrence wins — most recent record)
      if (!roadMeta[roadId] && rec.geo_point_2d) {
        roadMeta[roadId] = {
          label: (rec.libelle || "").replace(/_/g, " "),
          lat: rec.geo_point_2d.lat,
          lng: rec.geo_point_2d.lon
        };
      }

      dataCache[roadId].push({
        sensor_id: `PARIS-${sensorId}`,
        road_id: roadId,
        speed_kmh: Number(estimateSpeed(rec.k).toFixed(1)),
        flow_rate: Number((rec.q || 0).toFixed(1)),
        occupancy: Number(((rec.k || 0) / 100).toFixed(3)),
        original_ts: rec.t_1h,
        etat_trafic: rec.etat_trafic
      });
    }

    // Build active roads list (only roads with coordinates)
    activeRoads = Object.keys(dataCache).filter(r => roadMeta[r]?.lat);
    for (const r of activeRoads) cursors[r] = 0;

    usingRealData = activeRoads.length > 0;

    if (usingRealData) {
      console.log(`[simulator] fetched ${records.length} records → ${activeRoads.length} sensors with coordinates`);
      for (const r of activeRoads.slice(0, 10)) {
        const m = roadMeta[r];
        console.log(`  ${r}: ${dataCache[r].length} records (${m.label}, ${m.lat.toFixed(4)},${m.lng.toFixed(4)})`);
      }
      if (activeRoads.length > 10) console.log(`  ... and ${activeRoads.length - 10} more`);
    } else {
      console.warn("[simulator] no usable records with coordinates, falling back to random");
    }
  } catch (err) {
    console.error(`[simulator] Paris API error: ${err.message} — using fallback`);
    usingRealData = false;
  }
}

// ── Fallback: random data ──

const FALLBACK_ROADS = [
  { id: "SIM-CENTER-01", label: "Simulated Center", lat: 48.8566, lng: 2.3522 },
  { id: "SIM-NORTH-02", label: "Simulated North", lat: 48.8800, lng: 2.3400 },
  { id: "SIM-SOUTH-03", label: "Simulated South", lat: 48.8300, lng: 2.3500 },
  { id: "SIM-EAST-04", label: "Simulated East", lat: 48.8550, lng: 2.3800 },
  { id: "SIM-WEST-05", label: "Simulated West", lat: 48.8600, lng: 2.3100 }
];

function random(min, max) {
  return Math.random() * (max - min) + min;
}

function generateRandomPayload(road) {
  return {
    ts: new Date().toISOString(),
    sensor_id: road.id,
    road_id: road.id,
    speed_kmh: Number(random(12, 90).toFixed(1)),
    flow_rate: Number(random(10, 600).toFixed(1)),
    occupancy: Number(random(0.05, 0.95).toFixed(3)),
    label: road.label,
    lat: road.lat,
    lng: road.lng
  };
}

// ── Get next payload per road ──

function getPayload(roadId) {
  if (!usingRealData || !dataCache[roadId] || dataCache[roadId].length === 0) {
    const fallback = FALLBACK_ROADS.find(r => r.id === roadId) || FALLBACK_ROADS[0];
    return generateRandomPayload(fallback);
  }

  const records = dataCache[roadId];
  const idx = cursors[roadId] % records.length;
  cursors[roadId] = idx + 1;

  const rec = records[idx];
  const meta = roadMeta[roadId] || {};
  return {
    ts: new Date().toISOString(),
    sensor_id: rec.sensor_id,
    road_id: rec.road_id,
    speed_kmh: rec.speed_kmh,
    flow_rate: rec.flow_rate,
    occupancy: rec.occupancy,
    label: meta.label || "",
    lat: meta.lat || 0,
    lng: meta.lng || 0
  };
}

// ── MQTT client ──

const client = mqtt.connect(brokerUrl, { reconnectPeriod: 1000 });

client.on("connect", async () => {
  console.log(`[simulator] connected to ${brokerUrl}`);

  // Initial fetch
  await fetchParisData();

  // Publish loop
  setInterval(() => {
    const roads = usingRealData ? activeRoads : FALLBACK_ROADS.map(r => r.id);
    for (const roadId of roads) {
      const payload = getPayload(roadId);
      const topic = `cityflow/traffic/${payload.sensor_id}`;
      client.publish(topic, JSON.stringify(payload), { qos: 0 });
    }
    const src = usingRealData ? "Paris Open Data" : "random fallback";
    console.log(`[simulator] published ${roads.length} messages (${src})`);
  }, publishIntervalMs);

  // Refresh data every hour
  setInterval(fetchParisData, refreshIntervalMs);
});

client.on("error", (err) => {
  console.error("[simulator] mqtt error:", err.message);
});
