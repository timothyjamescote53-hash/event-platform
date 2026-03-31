-- ClickHouse schema for high-throughput event analytics
-- Optimized for time-series queries and aggregations

-- Raw events table with MergeTree engine
-- Partitioned by day for efficient time-range queries
CREATE TABLE IF NOT EXISTS events.raw_events
(
    tenant_id   LowCardinality(String),
    id          String,
    user_id     String,
    event_type  LowCardinality(String),
    source      LowCardinality(String),
    timestamp   Int64,        -- Unix millis
    version     UInt8,
    payload     String,       -- JSON
    labels      Map(String, String),
    -- Materialized columns for fast filtering
    event_date  Date          MATERIALIZED toDate(fromUnixTimestamp64Milli(timestamp)),
    event_hour  DateTime      MATERIALIZED toStartOfHour(fromUnixTimestamp64Milli(timestamp))
)
ENGINE = MergeTree()
PARTITION BY (tenant_id, event_date)
ORDER BY (tenant_id, event_type, timestamp)
TTL event_date + INTERVAL 90 DAY     -- Auto-expire data after 90 days
SETTINGS index_granularity = 8192;

-- Materialized view: per-minute aggregations (pre-computed for query performance)
CREATE MATERIALIZED VIEW IF NOT EXISTS events.events_per_minute
ENGINE = AggregatingMergeTree()
PARTITION BY toYYYYMM(window_start)
ORDER BY (tenant_id, event_type, window_start)
AS SELECT
    tenant_id,
    event_type,
    toStartOfMinute(fromUnixTimestamp64Milli(timestamp)) AS window_start,
    countState()          AS count_state,
    sumState(1.0)         AS sum_state,
    uniqState(user_id)    AS unique_users_state
FROM events.raw_events
GROUP BY tenant_id, event_type, window_start;

-- Query view for the materialized data
CREATE VIEW IF NOT EXISTS events.events_per_minute_view AS
SELECT
    tenant_id,
    event_type,
    window_start,
    countMerge(count_state)       AS count,
    sumMerge(sum_state)           AS sum,
    uniqMerge(unique_users_state) AS unique_users
FROM events.events_per_minute
GROUP BY tenant_id, event_type, window_start;

-- Revenue aggregation table
CREATE MATERIALIZED VIEW IF NOT EXISTS events.revenue_per_hour
ENGINE = SummingMergeTree()
PARTITION BY toYYYYMM(hour)
ORDER BY (tenant_id, hour)
AS SELECT
    tenant_id,
    toStartOfHour(fromUnixTimestamp64Milli(timestamp)) AS hour,
    sumIf(
        toFloat64OrZero(JSONExtractString(payload, 'amount')),
        event_type = 'purchase'
    ) AS revenue
FROM events.raw_events
GROUP BY tenant_id, hour;

-- Useful queries:

-- Events per user in last hour:
-- SELECT user_id, count() as cnt
-- FROM events.raw_events
-- WHERE tenant_id = 'acme' AND timestamp > (now() - INTERVAL 1 HOUR) * 1000
-- GROUP BY user_id ORDER BY cnt DESC LIMIT 10;

-- Revenue last 5 minutes:
-- SELECT sum(JSONExtractFloat(payload, 'amount'))
-- FROM events.raw_events
-- WHERE tenant_id = 'acme'
--   AND event_type = 'purchase'
--   AND timestamp > (toUnixTimestamp(now() - INTERVAL 5 MINUTE) * 1000);

-- Event rate per minute:
-- SELECT window_start, count, unique_users
-- FROM events.events_per_minute_view
-- WHERE tenant_id = 'acme' AND event_type = 'purchase'
-- ORDER BY window_start DESC LIMIT 60;
-- events
-- mat view
-- events
-- mat view
-- events
-- mat view
-- db
-- index
-- ttl
-- mat view
-- revenue
-- db
-- index
-- ttl
-- mat view
-- revenue
-- db
-- index
-- ttl
