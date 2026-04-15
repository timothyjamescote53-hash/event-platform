# Postmortem: 47-Minute Alert Delay Due to Kafka Partition Imbalance

**Severity:** P1  
**Duration:** 47 minutes delayed alerts (09:14–10:01 UTC, 2024-03-15)  
**Impact:** Alerts for tenant `acme` fired 47 minutes late. A real error spike went undetected for 47 minutes.  
**Author:** Platform Team  
**Status:** Complete

---

## Summary

A single tenant (`acme`) generating 80% of all events caused severe partition skew on our Kafka topic. The one partition handling `acme` traffic had a consumer lag of 280,000 messages while other partitions were empty. This caused `acme`'s events to be processed 47 minutes late, delaying all their alerts.

---

## Timeline

| Time (UTC) | Event |
|---|---|
| 09:14 | `acme` launches a marketing campaign — event rate spikes 8x |
| 09:14 | Kafka partition 3 (acme's partition) begins accumulating lag |
| 09:15 | Other 7 partitions are idle — processors sitting unused |
| 09:31 | `KafkaConsumerLagHigh` alert fires (lag > 50,000) |
| 09:35 | On-call engineer investigates — sees partition imbalance |
| 09:40 | Engineer scales processors to 8 replicas (= partition count) |
| 09:42 | All 8 processors assigned to partition 3 (Kafka rebalance) |
| 09:58 | Lag clears, real-time processing resumes |
| 10:01 | Delayed `acme` alerts fire — 47 min after actual error spike |

---

## Root Cause

Our Kafka partitioning strategy uses `tenant_id` as the partition key. This ensures per-tenant ordering but causes **hot partitions** when one tenant dominates traffic.

`acme` typically generates 20% of traffic. During the campaign, they generated 80%, saturating their single assigned partition faster than one consumer could process.

Meanwhile, 7 processors sat idle watching empty partitions.

---

## Contributing Factors

1. **No sub-tenant partitioning**: We didn't have a fallback to partition by `tenant_id:user_id` for large tenants
2. **HPA lag**: Kubernetes HPA scaling took 3 minutes to add processors — by then lag was already 50,000
3. **Alert threshold too high**: `KafkaConsumerLagHigh` fires at 50,000 — we were already 17 minutes behind by then

---

## Impact

- `acme` missed a real error spike for 47 minutes
- ~12,000 error events were processed late
- No data loss — events were retained in Kafka and eventually processed correctly

---

## Action Items

| Action | Owner | Due | Status |
|---|---|---|---|
| Implement per-tenant partition cap: tenants > 10% traffic use sub-key partitioning | @alice | Mar 22 | ✅ Done |
| Lower lag alert threshold to 10,000 (was 50,000) | @bob | Mar 18 | ✅ Done |
| Add Kafka lag to HPA trigger (scale on lag, not just CPU) | @charlie | Mar 25 | In Progress |
| Add Grafana panel: per-partition lag heatmap | @david | Mar 22 | ✅ Done |
| Implement partition rebalancing runbook | @alice | Mar 29 | Planned |

---

## Lessons Learned

**What went well:**
- No data loss — Kafka's durability worked as designed
- Root cause identified quickly (9 minutes)
- Fix (scaling processors) was effective

**What could be better:**
- Tenant isolation shouldn't rely on a single partition
- Alert thresholds need to account for lag-to-delay conversion
- HPA should react to consumer lag, not just CPU

---

## Prevention

Going forward, large tenants (>10% of traffic) will use compound partition keys (`tenant_id:shard`), distributing their load across multiple partitions while maintaining per-shard ordering.
<!-- v1 -->
<!-- v1 -->
<!-- v1 -->
<!-- timeline -->
<!-- actions -->
<!-- lessons -->
<!-- timeline -->
<!-- actions -->
<!-- lessons -->
<!-- timeline -->
<!-- actions -->
<!-- lessons -->
<!-- timeline -->
<!-- actions -->
<!-- lessons -->
