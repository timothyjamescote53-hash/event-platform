# Runbook: Debugging Kafka Consumer Lag

**Owner:** Platform Team  
**Trigger:** `KafkaConsumerLagHigh` alert (lag > 50,000) or `KafkaConsumerLagWarning` (lag > 10,000)  
**Severity:** P1 if lag > 50k (events delayed > 5 min), P2 if lag > 10k

---

## What is Consumer Lag?

Consumer lag = messages in Kafka topic − messages processed by consumers.

High lag means your stream processors are **falling behind** — events are being produced faster than they're being consumed. This causes:
- Delayed alerts (an anomaly from 10 min ago fires now)
- Stale dashboard metrics
- Potential OOM if lag grows unbounded

---

## 1. Assess Severity (0–2 min)

```bash
# Check current lag via Kafka UI
open http://localhost:8090  # Kafka UI

# Or via CLI
kubectl exec -n event-platform deploy/kafka -- \
  kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 \
  --describe --group stream-processor

# Output:
# TOPIC    PARTITION  CURRENT-OFFSET  LOG-END-OFFSET  LAG
# events   0          124500          174500          50000  ← high lag
# events   1          180200          180300          100    ← healthy
```

**Is lag growing or stable?**
```bash
# Watch lag over 30 seconds
watch -n 5 'kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --describe --group stream-processor | grep events'
```
- Growing → processors are down or too slow → **urgent**
- Stable → brief spike already recovering → monitor

---

## 2. Check Processor Health (2–5 min)

```bash
# Are processors running?
kubectl get pods -n event-platform -l app=processor-service

# Check for OOM or crash
kubectl describe pods -n event-platform -l app=processor-service | grep -A5 "Last State"

# Check logs for errors
kubectl logs -n event-platform deploy/processor-service --tail=100 | grep '"level":"error"'
```

**Common log errors:**

| Error | Cause | Fix |
|---|---|---|
| `kafka: connection refused` | Kafka broker down | Check Kafka pod |
| `context deadline exceeded` | ClickHouse slow | Check ClickHouse health |
| `OOM killed` | Memory leak or large event payloads | Increase memory limit or fix leak |
| `too many open files` | File descriptor leak | Restart pod, investigate |

---

## 3. Scale Up Processors (5–10 min)

**Quickest fix: add more consumers (up to partition count)**

```bash
# Current replicas
kubectl get deployment processor-service -n event-platform

# Scale up (max useful = number of Kafka partitions = 8 default)
kubectl scale deployment processor-service -n event-platform --replicas=8

# Watch lag decrease
watch -n 5 'kubectl exec -n event-platform deploy/kafka -- \
  kafka-consumer-groups.sh --bootstrap-server localhost:9092 \
  --describe --group stream-processor'
```

**Expected recovery rate:** Each additional processor should consume ~5,000–10,000 msgs/sec. With 50,000 lag and 4 extra processors: ~10–15 min to recover.

---

## 4. If Processors are Crashing

```bash
# Check for panic
kubectl logs -n event-platform <pod-name> --previous | tail -50

# Common panic: nil pointer in event deserialization
# Fix: check if a malformed event is in DLQ
kubectl exec -n event-platform deploy/kafka -- \
  kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 \
  --topic events.dlq --max-messages 5

# If DLQ has messages, inspect them
# Fix: patch the offending event schema, restart processors
```

---

## 5. If Kafka Broker is Slow

```bash
# Check broker metrics
kubectl exec -n event-platform deploy/kafka -- \
  kafka-topics.sh --bootstrap-server localhost:9092 --describe --topic events

# Check disk usage (Kafka is disk-bound)
kubectl exec -n event-platform deploy/kafka -- df -h /var/lib/kafka

# If disk > 80%: increase retention or add storage
kubectl patch configmap kafka-config -n event-platform \
  --patch '{"data": {"log.retention.hours": "48"}}'  # Reduce from 168h
```

---

## 6. Prevention

After resolving, create a ticket to:
- [ ] Review partition count (more partitions = more parallelism ceiling)
- [ ] Set Kubernetes HPA trigger on `kafka_consumer_lag` metric
- [ ] Add lag trending to Grafana dashboard
- [ ] Review processor memory/CPU limits — may need tuning

---

## Escalation

If lag is not recovering after 20 min:
1. Enable read-from-end mode (skip old events) — **use with caution, causes data loss**
2. Increase Kafka partition count (requires topic recreation)
3. Page on-call lead
<!-- v1 -->
<!-- v1 -->
<!-- v1 -->
<!-- assess -->
<!-- causes -->
<!-- escalation -->
<!-- metrics -->
<!-- assess -->
<!-- causes -->
<!-- escalation -->
<!-- metrics -->
<!-- assess -->
<!-- causes -->
<!-- escalation -->
<!-- metrics -->
