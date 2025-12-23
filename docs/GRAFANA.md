# ION Grafana Dashboard

This directory contains a pre-built Grafana dashboard for monitoring services using the ION logging library.

## Dashboard Features

### Overview Section
- **Log Level Distribution** - Pie chart showing debug/info/warn/error breakdown
- **Total Logs** - Count of all log entries in the time range
- **Errors / Warnings** - Quick counts with color-coded thresholds
- **Error Rate** - Percentage of error logs vs total
- **Logs by Component** - Distribution across your application components
- **Logs by Service** - Multi-service comparison

### Log Volume Over Time
- **Volume by Level** - Stacked bar chart showing log activity over time
- **Volume by Component** - Line chart for component-level analysis
- **Volume by Service** - Multi-service trend comparison

### Blockchain Metrics
- **Transaction Log Events** - Timeline of logs with `tx_hash` field
- **Logs by Shard** - Distribution across blockchain shards
- **Recent Transactions** - Table with tx_hash, status, latency, shard

### Performance & Latency
- **Average/Max Latency** - Stats for `latency_ms` field
- **Average Duration** - Stats for `duration_ms` field
- **Latency/Duration Histograms** - Distribution visualization

### Errors & Warnings
- **Errors by Component** - Timeline showing which components generate errors
- **Recent Errors & Warnings** - Table with full log details

### Trace Correlation
- **Logs by Trace ID** - Filter all logs by trace_id for distributed tracing

### Raw Logs
- **All Logs** - Live log stream with filters

## Installation

### 1. Import to Grafana

1. Open Grafana → Dashboards → Import
2. Upload `grafana-dashboard.json` or paste its contents
3. Select your Loki datasource
4. Click "Import"

### 2. Prerequisites

- **Loki** as log storage (via OTEL Collector or direct)
- **OTEL Collector** receiving logs from ION with Loki exporter configured

Example OTEL Collector config for Loki:

```yaml
exporters:
  loki:
    endpoint: http://loki:3100/loki/api/v1/push
    labels:
      attributes:
        service_name: ""
        level: ""

service:
  pipelines:
    logs:
      receivers: [otlp]
      processors: [batch]
      exporters: [loki]
```

### 3. Dashboard Variables

The dashboard includes these configurable variables:

| Variable | Description |
|----------|-------------|
| `datasource` | Loki datasource to query |
| `service` | Filter by service_name (multi-select) |
| `level` | Filter by log level (multi-select) |
| `component` | Filter by component name (multi-select) |
| `trace_id` | Text input to filter by trace ID |

## ION Field Mapping

The dashboard expects these structured fields from ION:

| Field | Description | Source |
|-------|-------------|--------|
| `level` | Log level (debug/info/warn/error) | Core |
| `msg` | Log message | Core |
| `service` | Service name | `Config.ServiceName` |
| `version` | Service version | `Config.Version` |
| `component` | Named logger component | `logger.Named()` |
| `trace_id` | OpenTelemetry trace ID | Context |
| `span_id` | OpenTelemetry span ID | Context |
| `tx_hash` | Transaction hash | `fields.TxHash()` |
| `shard_id` | Shard identifier | `fields.ShardID()` |
| `slot` | Blockchain slot | `fields.Slot()` |
| `latency_ms` | Operation latency | `fields.LatencyMs()` |
| `duration_ms` | Operation duration | `fields.DurationMs()` |
| `error` | Error message | `logger.Error()` |

## Customization

Feel free to modify the dashboard for your needs:

1. **Add new panels** for custom fields you're logging
2. **Adjust thresholds** in the stat panels (errors, latency)
3. **Create alerts** based on error rates or latency spikes
4. **Link to Tempo/Jaeger** for trace visualization

## Troubleshooting

**No data showing?**
- Verify your OTEL Collector is receiving logs
- Check Loki is ingesting data: `logcli query '{service_name=~".+"}' --limit 5`
- Ensure `service_name` label is set in your OTEL Collector config

**Filters not populating?**
- Variables require at least one log entry with that label
- Check the LogQL queries in variable definitions
