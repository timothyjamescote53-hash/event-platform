#!/usr/bin/env bash
# bootstrap.sh — Initialize the repo and push to GitHub
# Usage: ./bootstrap.sh <github-username> [repo-name]
set -euo pipefail

GITHUB_USER="${1:?Usage: ./bootstrap.sh <github-username> [repo-name]}"
REPO_NAME="${2:-event-platform}"

echo "🚀 Bootstrapping $REPO_NAME for GitHub user: $GITHUB_USER"

# ── Prerequisites check ────────────────────────────────────────────────────────
for cmd in git go docker gh; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "❌ Required tool not found: $cmd"
    exit 1
  fi
done

echo "✅ Prerequisites OK"

# ── Git init ───────────────────────────────────────────────────────────────────
if [ ! -d ".git" ]; then
  git init
  git branch -M main
fi

# ── Replace placeholder org with real username ─────────────────────────────────
echo "🔧 Updating module paths to github.com/$GITHUB_USER/$REPO_NAME ..."
find . -type f \( -name "*.go" -o -name "*.mod" -o -name "*.yaml" -o -name "*.yml" -o -name "*.md" \) \
  -not -path "./.git/*" \
  -exec sed -i.bak "s|github.com/yourorg/event-platform|github.com/$GITHUB_USER/$REPO_NAME|g" {} \;
find . -name "*.bak" -delete

sed -i.bak "s|yourorg|$GITHUB_USER|g" \
  infrastructure/kubernetes/services/deployments.yaml \
  .github/workflows/ci-cd.yml 2>/dev/null || true
find . -name "*.bak" -delete

echo "✅ Module paths updated"

# ── Create GitHub repo ─────────────────────────────────────────────────────────
echo "📦 Creating GitHub repository: $GITHUB_USER/$REPO_NAME ..."
gh repo create "$GITHUB_USER/$REPO_NAME" \
  --public \
  --description "Real-Time Event Processing & Intelligence Platform — Go, Kafka, ClickHouse, Kubernetes, AI anomaly detection" \
  --homepage "https://github.com/$GITHUB_USER/$REPO_NAME" || echo "Repo may already exist, continuing..."

git remote remove origin 2>/dev/null || true
git remote add origin "https://github.com/$GITHUB_USER/$REPO_NAME.git"

echo "✅ GitHub repo created"

# ── Initial commit ─────────────────────────────────────────────────────────────
git add .
git commit -m "feat: initial event platform scaffold

Services:
- Ingestion Service: REST API, batching, idempotency deduplication, Kafka producer
- Stream Processor: Kafka consumers, tumbling/sliding windows, z-score anomaly detection
- Query API: real-time analytics, Redis hot data, ClickHouse historical queries
- Alerting Service: rule engine, Slack notifications, smart deduplication
- AI Analyzer: LLM anomaly summaries, alert deduplication, predictive scaling

Infrastructure:
- Kafka (8 partitions, snappy compression, DLQ)
- ClickHouse (MergeTree, materialized views for pre-aggregation)
- Redis (deduplication, window state, rate limiting)
- Docker Compose with full observability stack
- Kubernetes manifests with custom HPA (lag-based scaling)
- GitHub Actions CI/CD with load test gate

Observability:
- Prometheus metrics (consumer lag, throughput, error rates)
- OpenTelemetry tracing end-to-end
- Grafana dashboards
- Jaeger distributed tracing

Docs:
- ADR-001: Kafka vs NATS
- ADR-002: Stream vs Batch processing
- Runbooks: consumer lag debugging, event loss recovery
- Postmortem: Kafka partition imbalance incident"

git push -u origin main

echo ""
echo "🎉 Done! Your platform is live at:"
echo "   https://github.com/$GITHUB_USER/$REPO_NAME"
echo ""
echo "Next steps:"
echo "  1. docker compose up --build     → Run locally"
echo "  2. open http://localhost:8090    → Kafka UI"
echo "  3. open http://localhost:3000    → Grafana (admin/admin)"
echo "  4. open http://localhost:16686   → Jaeger traces"
echo "  5. k6 run infrastructure/load-testing/k6-load-test.js"
echo ""
echo "Quick API test:"
echo "  curl -X POST http://localhost:8080/api/v1/events \\"
echo "    -H 'Content-Type: application/json' \\"
echo "    -H 'X-Tenant-ID: acme' \\"
echo "    -d '{\"event_type\":\"purchase\",\"payload\":{\"amount\":99.99}}'"
