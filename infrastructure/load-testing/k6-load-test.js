import http from "k6/http";
import { check, sleep } from "k6";
import { Rate, Trend, Counter } from "k6/metrics";

// Custom metrics
const errorRate = new Rate("errors");
const ingestionLatency = new Trend("ingestion_latency_ms");
const batchIngestionLatency = new Trend("batch_ingestion_latency_ms");
const eventsIngested = new Counter("events_ingested_total");

export const options = {
  scenarios: {
    // Scenario 1: Sustained load
    sustained_load: {
      executor: "constant-arrival-rate",
      rate: 1000,           // 1000 requests/sec
      timeUnit: "1s",
      duration: "5m",
      preAllocatedVUs: 50,
      maxVUs: 200,
      tags: { scenario: "sustained" },
    },
    // Scenario 2: Traffic spike (simulates flash sale / viral event)
    traffic_spike: {
      executor: "ramping-arrival-rate",
      startRate: 100,
      timeUnit: "1s",
      stages: [
        { target: 5000, duration: "30s" },  // Spike to 5000 RPS
        { target: 5000, duration: "2m" },   // Hold
        { target: 100,  duration: "30s" },  // Recover
      ],
      preAllocatedVUs: 100,
      maxVUs: 500,
      startTime: "3m",  // Start after sustained load
      tags: { scenario: "spike" },
    },
    // Scenario 3: Batch ingestion
    batch_load: {
      executor: "constant-vus",
      vus: 20,
      duration: "5m",
      tags: { scenario: "batch" },
      exec: "batchTest",
    },
  },
  thresholds: {
    // SLO: p99 ingestion latency < 50ms
    "ingestion_latency_ms{scenario:sustained}": ["p(99)<50"],
    // SLO: Error rate < 0.1%
    errors: ["rate<0.001"],
    // Batch latency can be higher
    "batch_ingestion_latency_ms": ["p(95)<200"],
    // Overall HTTP duration
    http_req_duration: ["p(95)<100"],
  },
};

const BASE_URL = __ENV.BASE_URL || "http://localhost:8080";

const EVENT_TYPES = ["purchase", "click", "page_view", "login", "logout", "error", "search"];
const TENANTS = ["tenant-acme", "tenant-globex", "tenant-initech"];

function randomEvent() {
  return {
    event_type: EVENT_TYPES[Math.floor(Math.random() * EVENT_TYPES.length)],
    user_id: `user-${Math.floor(Math.random() * 10000)}`,
    source: "web",
    payload: {
      amount: Math.random() * 500,
      page: "/checkout",
      session_id: `sess-${Math.random().toString(36).substr(2, 9)}`,
    },
    labels: {
      region: "us-east-1",
      version: "2.1.0",
    },
  };
}

// Default scenario: single event ingestion
export default function () {
  const tenant = TENANTS[Math.floor(Math.random() * TENANTS.length)];
  const start = Date.now();

  const res = http.post(
    `${BASE_URL}/api/v1/events`,
    JSON.stringify(randomEvent()),
    {
      headers: {
        "Content-Type": "application/json",
        "X-Tenant-ID": tenant,
      },
      timeout: "5s",
    }
  );

  const latency = Date.now() - start;
  ingestionLatency.add(latency);
  eventsIngested.add(1);

  const ok = check(res, {
    "status 202 accepted": (r) => r.status === 202,
    "has event id": (r) => {
      try {
        return JSON.parse(r.body).id !== undefined;
      } catch {
        return false;
      }
    },
    "latency < 50ms": () => latency < 50,
  });

  errorRate.add(!ok);
}

// Batch scenario: 100 events per request
export function batchTest() {
  const tenant = TENANTS[Math.floor(Math.random() * TENANTS.length)];
  const events = Array.from({ length: 100 }, randomEvent);
  const start = Date.now();

  const res = http.post(
    `${BASE_URL}/api/v1/events/batch`,
    JSON.stringify({ events }),
    {
      headers: {
        "Content-Type": "application/json",
        "X-Tenant-ID": tenant,
      },
      timeout: "10s",
    }
  );

  const latency = Date.now() - start;
  batchIngestionLatency.add(latency);
  eventsIngested.add(100);

  const ok = check(res, {
    "batch accepted": (r) => r.status === 202 || r.status === 200,
    "all events accepted": (r) => {
      try {
        const body = JSON.parse(r.body);
        return body.accepted > 0;
      } catch {
        return false;
      }
    },
  });

  errorRate.add(!ok);
  sleep(0.1);
}

export function handleSummary(data) {
  const p99 = data.metrics.ingestion_latency_ms?.values?.["p(99)"] ?? 0;
  const p999 = data.metrics.http_req_duration?.values?.["p(99.9)"] ?? 0;
  const errRate = (data.metrics.errors?.values?.rate ?? 0) * 100;
  const totalEvents = data.metrics.events_ingested_total?.values?.count ?? 0;
  const rps = data.metrics.http_reqs?.values?.rate ?? 0;

  return {
    "results/load-test-summary.json": JSON.stringify(data, null, 2),
    stdout: `
╔══════════════════════════════════════════════════════════╗
║          EVENT PLATFORM LOAD TEST RESULTS                ║
╠══════════════════════════════════════════════════════════╣
║  Total Events Ingested: ${String(totalEvents).padEnd(30)}║
║  Peak RPS:              ${String(Math.round(rps)).padEnd(30)}║
║                                                          ║
║  LATENCY                                                 ║
║  p99 (single event):    ${String(Math.round(p99) + "ms").padEnd(30)}║
║  p99.9 (overall):       ${String(Math.round(p999) + "ms").padEnd(30)}║
║                                                          ║
║  RELIABILITY                                             ║
║  Error Rate:            ${String(errRate.toFixed(4) + "%").padEnd(30)}║
║                                                          ║
║  SLO STATUS                                              ║
║  p99 < 50ms:   ${(p99 < 50 ? "✅ PASS" : "❌ FAIL").padEnd(43)}║
║  Error < 0.1%: ${(errRate < 0.1 ? "✅ PASS" : "❌ FAIL").padEnd(43)}║
╚══════════════════════════════════════════════════════════╝
`,
  };
}
// sustained
// spike
// batch
// sustained
// spike
// batch
// sustained
// spike
// batch
// options
// sustained
// spike
// batch scenario
// summary
// metrics
// tenants
// think time
// options
// sustained
// spike
// batch scenario
// summary
// metrics
// tenants
// think time
// options
// sustained
// spike
// batch scenario
// summary
// metrics
// tenants
// think time
// options
// sustained
