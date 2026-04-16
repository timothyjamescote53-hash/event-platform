# ADR-001: Kafka as the Event Streaming Backbone

**Date:** 2024-03-01  
**Status:** Accepted  
**Deciders:** Platform Team

---

## Context

We needed an event bus that could handle millions of events per second with durability, replay capability, and consumer group semantics. The two main candidates were **Apache Kafka** and **NATS JetStream**.

---

## Decision

Use **Apache Kafka** as the primary event streaming backbone.

---

## Comparison

| Concern | Kafka | NATS JetStream |
|---|---|---|
| Throughput | Millions/sec per broker | Hundreds of thousands/sec |
| Durability | Disk-based, configurable retention | Memory-first with optional disk |
| Replay | Full log replay (any offset) | Limited retention window |
| Consumer groups | First-class, partition-level | Supported but less mature |
| Ordering | Guaranteed per partition | Guaranteed per subject |
| Ops complexity | Higher (ZooKeeper/KRaft) | Much simpler |
| Ecosystem | Kafka Connect, ksqlDB, Flink | Growing but smaller |
| Latency | ~5–10ms | ~1–2ms |

**Why Kafka won:**
1. **Replay is critical** — if a processor bug corrupts aggregations, we need to reprocess from any point in history
2. **Partition-based scaling** — we can add consumers up to the partition count (8 partitions → 8 max parallel workers)
3. **Ecosystem** — Kafka Connect for CDC pipelines, Flink for complex CEP in future phases
4. **Log retention** — 7-day retention lets us debug production issues days after the fact

**Where NATS would have been better:**
- Pure latency (1ms vs 10ms) — acceptable trade-off for our use case
- Operational simplicity — Kafka requires ZooKeeper or KRaft; NATS is a single binary

---

## Partitioning Strategy

Partition key = `tenant_id`. This ensures:
- All events from the same tenant land on the same partition
- Per-tenant ordering is guaranteed
- Consumer groups process one tenant's events sequentially

Trade-off: hot tenants can cause partition skew. Mitigation: monitor per-partition lag; repartition large tenants with a sub-key like `tenant_id:user_id`.

---

## Delivery Semantics

We use **at-least-once** delivery:
- Producers use `RequireOne` acks (not `RequireAll` — acceptable for speed)
- Consumers commit offsets after processing (not before)
- Idempotency key deduplication in Redis handles duplicate delivery

**Why not exactly-once?**
Kafka's exactly-once (transactional producers + EOS consumers) adds ~30% latency overhead. Our Redis deduplication layer provides the same guarantee at lower cost.

---

## Consequences

- Kafka adds operational overhead — managed via AWS MSK in production
- Partition count is set at topic creation — we provision 16 partitions to allow future scaling
- ZooKeeper dependency replaced by KRaft mode (Kafka 3.x+)
<!-- v1 -->
<!-- v1 -->
<!-- v1 -->
<!-- context -->
<!-- comparison -->
<!-- decision -->
<!-- partitioning -->
<!-- delivery -->
<!-- context -->
<!-- comparison -->
<!-- decision -->
<!-- partitioning -->
<!-- delivery -->
