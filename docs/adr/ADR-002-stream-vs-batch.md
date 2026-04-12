# ADR-002: Stream Processing over Batch for Real-Time Analytics

**Date:** 2024-03-01  
**Status:** Accepted

---

## Context

Should we process events in real-time as they arrive (stream processing) or accumulate them and process periodically (batch processing)?

## Decision

Use **stream processing** for real-time aggregations and alerting, with **ClickHouse materialized views** for pre-aggregated historical queries.

---

## Architecture

```
Events → Kafka → Stream Processor (Go workers)
                    ├── Redis Windows  (hot: last 2h, sub-second latency)
                    └── ClickHouse     (cold: full history, second latency)
```

## Tradeoffs

| Dimension | Stream | Batch |
|---|---|---|
| Latency | Sub-second | Minutes to hours |
| Complexity | Higher (stateful, windowing) | Lower |
| Cost | Continuous compute | Bursty compute |
| Consistency | Eventual (processing lag) | Strong (after batch completes) |
| Late arrivals | Requires watermarks | Handled naturally |

**Why stream won:**
- Alerting requirement: "fire alert when error rate > X in last 1 minute" — impossible with batch
- Dashboard requirement: "revenue last 5 minutes" — batch would be stale
- Anomaly detection must happen as events arrive, not 10 minutes later

**Why we kept ClickHouse (hybrid approach):**
- Historical queries ("events per user last 30 days") are expensive to serve from Redis
- ClickHouse materialized views give us pre-aggregated OLAP performance
- ClickHouse ingests directly from Kafka via the Kafka table engine

---

## Windowing Strategy

We implement **tumbling windows** (non-overlapping, fixed-size):
- 1-minute windows for alerting
- 5-minute windows for dashboards
- 1-hour windows for trend analysis

We also support **sliding windows** for anomaly detection:
- 5-minute window sliding every 30 seconds
- Gives smoother signal for z-score calculation

**Why not Apache Flink?**
Flink provides richer windowing (sessions, watermarks, late data handling) but adds significant operational complexity. Our Go-based window manager covers 90% of use cases. We revisit Flink if we need session windows or complex event processing.

---

## Late Arrival Handling

Events with timestamps more than 5 minutes in the past are:
1. Still ingested and stored in ClickHouse (for historical accuracy)
2. **Not** added to Redis windows (window has already closed)
3. Flagged with `late_arrival: true` label for monitoring

This is acceptable: late arrivals are typically <0.1% of traffic and are caused by mobile clients with poor connectivity.
<!-- v1 -->
<!-- v1 -->
<!-- v1 -->
<!-- context -->
<!-- tradeoffs -->
<!-- windowing -->
<!-- late arrival -->
<!-- context -->
<!-- tradeoffs -->
<!-- windowing -->
<!-- late arrival -->
<!-- context -->
<!-- tradeoffs -->
<!-- windowing -->
<!-- late arrival -->
<!-- context -->
