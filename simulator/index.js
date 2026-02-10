const mqtt = require("mqtt");

const brokerUrl = process.env.MQTT_URL || "mqtt://localhost:1883";
const publishIntervalMs = Number(process.env.PUBLISH_INTERVAL_MS || 2000);
const refreshIntervalMs = Number(process.env.REFRESH_INTERVAL_MS || 3600000); // 1h

// ── Paris Open Data: real sensor → CityFlow road mapping ──

const API_BASE = "https://opendata.paris.fr/api/explore/v2.1/catalog/datasets/comptages-routiers-permanents/records";

const ROAD_MAPPING = {
  "1643": "RING-NORTH-12",    // Chapelle — north Paris
  "1645": "RING-NORTH-12",
  "4841": "RING-SOUTH-09",    // Av Daumesnil — south-east Paris
  "4843": "RING-SOUTH-09",
  "5041": "RING-SOUTH-09",
  "27":   "CITY-CENTER-01",   // Rue de Rivoli — central Paris
  "23":   "CITY-CENTER-01",
  "28":   "CITY-CENTER-01",
  "6556": "AIRPORT-AXIS-03",  // Av Jean Jaurès — north-east (toward airports)
  "4896": "AIRPORT-AXIS-03",
  "4898": "AIRPORT-AXIS-03",
  "1408": "UNIVERSITY-LOOP-07", // Av Gambetta — east Paris
  "1462": "UNIVERSITY-LOOP-07"
};

const SENSOR_IDS = Object.keys(ROAD_MAPPING);

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
let usingRealData = false;

const ROADS = [
  "RING-NORTH-12", "RING-SOUTH-09", "CITY-CENTER-01",
  "AIRPORT-AXIS-03", "UNIVERSITY-LOOP-07"
];

ROADS.forEach(r => { dataCache[r] = []; cursors[r] = 0; });

// ── Fetch from Paris Open Data ──

async function fetchParisData() {
  const iuList = SENSOR_IDS.map(id => `'${id}'`).join(",");
  const where = `q is not null and k is not null and iu_ac in (${iuList})`;
  const url = `${API_BASE}?where=${encodeURIComponent(where)}&select=iu_ac,libelle,q,k,etat_trafic,t_1h&order_by=t_1h+desc&limit=100`;

  try {
    const res = await fetch(url, {
      headers: { "User-Agent": "CityFlow-Simulator/1.0" }
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    const json = await res.json();
    const records = json.results || [];

    if (records.length === 0) {
      console.warn("[simulator] Paris API returned 0 records, keeping previous cache");
      return;
    }

    // Group by road
    ROADS.forEach(r => { dataCache[r] = []; });

    for (const rec of records) {
      const roadId = ROAD_MAPPING[String(rec.iu_ac)];
      if (!roadId) continue;

      dataCache[roadId].push({
        sensor_id: `PARIS-${rec.iu_ac}`,
        road_id: roadId,
        speed_kmh: Number(estimateSpeed(rec.k).toFixed(1)),
        flow_rate: Number((rec.q || 0).toFixed(1)),
        occupancy: Number(((rec.k || 0) / 100).toFixed(3)),
        original_ts: rec.t_1h,
        libelle: rec.libelle,
        etat_trafic: rec.etat_trafic
      });
    }

    const total = Object.values(dataCache).reduce((s, a) => s + a.length, 0);
    usingRealData = total > 0;

    if (usingRealData) {
      ROADS.forEach(r => { cursors[r] = 0; });
      console.log(`[simulator] fetched ${total} records from Paris Open Data`);
      ROADS.forEach(r => {
        console.log(`  ${r}: ${dataCache[r].length} records`);
      });
    } else {
      console.warn("[simulator] no usable records, falling back to random");
    }
  } catch (err) {
    console.error(`[simulator] Paris API error: ${err.message} — using fallback`);
    usingRealData = false;
  }
}

// ── Fallback: random data (original behavior) ──

function random(min, max) {
  return Math.random() * (max - min) + min;
}

function generateRandomPayload(roadId) {
  return {
    ts: new Date().toISOString(),
    sensor_id: `SIM-${roadId.substring(0, 4)}`,
    road_id: roadId,
    speed_kmh: Number(random(12, 90).toFixed(1)),
    flow_rate: Number(random(10, 600).toFixed(1)),
    occupancy: Number(random(0.05, 0.95).toFixed(3))
  };
}

// ── Get next payload per road ──

function getPayload(roadId) {
  if (!usingRealData || dataCache[roadId].length === 0) {
    return generateRandomPayload(roadId);
  }

  const records = dataCache[roadId];
  const idx = cursors[roadId] % records.length;
  cursors[roadId] = idx + 1;

  const rec = records[idx];
  return {
    ts: new Date().toISOString(),
    sensor_id: rec.sensor_id,
    road_id: rec.road_id,
    speed_kmh: rec.speed_kmh,
    flow_rate: rec.flow_rate,
    occupancy: rec.occupancy
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
    for (const roadId of ROADS) {
      const payload = getPayload(roadId);
      const topic = `cityflow/traffic/${payload.sensor_id}`;
      client.publish(topic, JSON.stringify(payload), { qos: 0 });
    }
    const src = usingRealData ? "Paris Open Data" : "random fallback";
    console.log(`[simulator] published ${ROADS.length} messages (${src})`);
  }, publishIntervalMs);

  // Refresh data every hour
  setInterval(fetchParisData, refreshIntervalMs);
});

client.on("error", (err) => {
  console.error("[simulator] mqtt error:", err.message);
});
