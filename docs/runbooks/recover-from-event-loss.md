# Runbook: Recovering from Event Loss

**Owner:** Platform Team  
**Trigger:** Missing data in dashboards, DLQ growing, or customer reports of missing events  
**Severity:** P0 if revenue events lost, P1 for all other event types

---

## Types of Event Loss

| Scenario | Detectable By | Data Recoverable? |
|---|---|---|
| Events never reached ingestion service | Client-side logs, ingestion count drop | Only if client retried |
| Events accepted but Kafka write failed | `ingestion_events_failed_total` metric | No (lost in buffer) |
| Events in Kafka but processor crashed | Consumer lag + DLQ | Yes — reprocess from offset |
| Processor ran but ClickHouse write failed | DLQ, ClickHouse error logs | Yes — replay from Kafka |
| ClickHouse data corrupted/deleted | Query returns wrong results | Depends on backup |

---

## Scenario A: Events in DLQ (Most Common)

Dead letter queue = events that failed processing after retries.

```bash
# Check DLQ size
kubectl exec -n event-platform deploy/kafka -- \
  kafka-run-class.sh kafka.tools.GetOffsetShell \
  --broker-list localhost:9092 --topic events.dlq

# Inspect failed events
kubectl exec -n event-platform deploy/kafka -- \
  kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic events.dlq \
  --from-beginning --max-messages 10

# Each DLQ message contains:
# { "original_event": {...}, "error": "...", "failed_at": ..., "attempts": 3 }
```

**Reprocessing DLQ:**
```bash
# 1. Fix the root cause first (schema bug, ClickHouse down, etc.)
# 2. Re-publish DLQ events back to main topic
kubectl exec -n event-platform deploy/kafka -- \
  kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic events.dlq --from-beginning \
  | jq -r '.original_event' \
  | kafka-console-producer.sh \
  --broker-list localhost:9092 \
  --topic events.recovery
```

---

## Scenario B: Replay from Kafka (Reprocessing)

Kafka retains events for 7 days. You can replay any time window.

```bash
# Reset consumer group to a specific timestamp
# (replays all events from that time forward)
kubectl exec -n event-platform deploy/kafka -- \
  kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --group stream-processor \
  --reset-offsets \
  --to-datetime 2024-03-01T10:00:00.000 \
  --topic events \
  --execute

# Now restart processors — they'll reprocess from that point
kubectl rollout restart deployment/processor-service -n event-platform

# Monitor reprocessing lag
watch -n 5 'kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --describe --group stream-processor'
```

⚠️ **Warning:** Reprocessing will re-trigger alerts and re-update aggregations. Consider:
- Disabling alert notifications during replay: `kubectl set env deploy/alerting-service NOTIFICATIONS_ENABLED=false`
- Using a separate consumer group for replay to avoid affecting live processing

---

## Scenario C: ClickHouse Data Loss

```bash
# Check ClickHouse table health
kubectl exec -n event-platform deploy/clickhouse -- \
  clickhouse-client --query "
    SELECT table, total_rows, total_bytes
    FROM system.tables
    WHERE database = 'events'
  "

# Check for data gaps
kubectl exec -n event-platform deploy/clickhouse -- \
  clickhouse-client --query "
    SELECT toDate(fromUnixTimestamp64Milli(timestamp)) as day, count()
    FROM events.raw_events
    WHERE tenant_id = 'acme'
    GROUP BY day
    ORDER BY day DESC
    LIMIT 14
  "
```

**If gap detected — restore from Kafka:**
1. Identify the time range of missing data
2. Run replay procedure from Scenario B for that window
3. Verify data restored: re-run the gap detection query

**If Kafka retention expired (> 7 days old):**
- Check S3 backups: `aws s3 ls s3://your-bucket/kafka-backups/`
- Restore from ClickHouse snapshot if available

---

## Communication Template

```
[DATA INCIDENT] We detected missing {event_type} events for tenant {tenant_id}
between {start_time} and {end_time} UTC.

Root cause: {cause}
Impact: {N} events affected (~{estimate} revenue events)
Status: Reprocessing in progress, ETA {time}

Dashboard metrics will be updated once reprocessing completes.
```

---

## Post-Incident

- [ ] Determine root cause and timeline
- [ ] Quantify events lost (count, types, tenants)
- [ ] Assess business impact (any revenue events?)
- [ ] File postmortem (use `docs/postmortems/TEMPLATE.md`)
- [ ] Add monitoring to detect this scenario earlier next time
<!-- v1 -->
<!-- v1 -->
<!-- v1 -->
<!-- dlq -->
<!-- replay -->
<!-- comms -->
<!-- dlq -->
<!-- replay -->
<!-- comms -->
<!-- dlq -->
