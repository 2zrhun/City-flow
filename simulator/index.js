const mqtt = require("mqtt");

const brokerUrl = process.env.MQTT_URL || "mqtt://localhost:1883";
const publishIntervalMs = Number(process.env.PUBLISH_INTERVAL_MS || 2000);
const sensorCount = Number(process.env.SENSOR_COUNT || 10);

const roads = [
  "RING-NORTH-12",
  "RING-SOUTH-09",
  "CITY-CENTER-01",
  "AIRPORT-AXIS-03",
  "UNIVERSITY-LOOP-07"
];

function random(min, max) {
  return Math.random() * (max - min) + min;
}

function generatePayload(sensorId) {
  const roadId = roads[Math.floor(Math.random() * roads.length)];

  return {
    ts: new Date().toISOString(),
    sensor_id: `S-${String(sensorId).padStart(3, "0")}`,
    road_id: roadId,
    speed_kmh: Number(random(12, 90).toFixed(1)),
    flow_rate: Number(random(10, 120).toFixed(1)),
    occupancy: Number(random(0.1, 0.98).toFixed(2))
  };
}

const client = mqtt.connect(brokerUrl, {
  reconnectPeriod: 1000
});

client.on("connect", () => {
  console.log(`[simulator] connected to ${brokerUrl}`);

  setInterval(() => {
    for (let i = 1; i <= sensorCount; i += 1) {
      const payload = generatePayload(i);
      const topic = `cityflow/traffic/${payload.sensor_id}`;
      client.publish(topic, JSON.stringify(payload), { qos: 0 });
    }
    console.log(`[simulator] published ${sensorCount} messages`);
  }, publishIntervalMs);
});

client.on("error", (err) => {
  console.error("[simulator] mqtt error:", err.message);
});
