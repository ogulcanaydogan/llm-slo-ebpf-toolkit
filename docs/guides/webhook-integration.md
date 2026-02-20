# Webhook Integration Guide

The webhook exporter delivers incident attribution events to HTTP endpoints with HMAC-SHA256 signing and retry support.

## Runtime Wiring

Webhook delivery is wired through `cmd/attributor` and reads defaults from `config/toolkit.yaml`.

```bash
go run ./cmd/attributor \
  --input artifacts/fault-replay/fault_samples.jsonl \
  --out artifacts/attribution/predictions.jsonl \
  --attribution-mode bayes \
  --webhook-enabled \
  --webhook-url https://your-endpoint.example.com/webhook \
  --webhook-secret "$WEBHOOK_SECRET"
```

Default behavior is fail-open (non-blocking): delivery errors are logged and the command still exits successfully.  
Enable strict mode for CI gates:

```bash
go run ./cmd/attributor \
  --input artifacts/fault-replay/fault_samples.jsonl \
  --webhook-enabled \
  --webhook-url https://your-endpoint.example.com/webhook \
  --webhook-strict
```

## Supported Formats

| Format | Target | Content-Type |
|--------|--------|-------------|
| `generic` | Any HTTP endpoint | `application/json` |
| `pagerduty` | PagerDuty Events API v2 | `application/json` |
| `opsgenie` | Opsgenie Alert API | `application/json` |

## Configuration

### Via toolkit.yaml

```yaml
webhook:
  enabled: true
  url: "https://your-endpoint.example.com/webhook"
  secret: "your-hmac-sha256-secret"
  format: "generic"
  timeout_ms: 5000
```

### Via Helm Values

```bash
helm install llm-slo-agent charts/llm-slo-agent \
  --set webhook.enabled=true \
  --set webhook.url=https://events.pagerduty.com/v2/enqueue \
  --set webhook.secret=your-secret \
  --set webhook.format=pagerduty
```

## PagerDuty Setup

1. Create a PagerDuty service with an Events API v2 integration.
2. Copy the integration key (routing key).
3. Configure the webhook:

```yaml
webhook:
  enabled: true
  url: "https://events.pagerduty.com/v2/enqueue"
  format: "pagerduty"
```

The PagerDuty payload maps:
- `routing_key`: Set via the PagerDuty integration (pass as part of your routing layer)
- `event_action`: `trigger`
- `severity`: Derived from burn rate (critical > 5.0, error > 2.0, warning > 1.0, info otherwise)
- `summary`: Incident ID, fault domain, and cluster
- `custom_details`: Full incident attribution with evidence and SLO impact

## Opsgenie Setup

1. Create an Opsgenie API integration.
2. Configure the webhook:

```yaml
webhook:
  enabled: true
  url: "https://api.opsgenie.com/v2/alerts"
  format: "opsgenie"
```

The Opsgenie payload maps:
- `alias`: Incident ID
- `priority`: P1 (burn rate > 5.0), P2 (> 2.0), P3 (otherwise)
- `tags`: Fault domain, cluster, service
- `details`: Signal evidence and SLO impact metrics

## HMAC Verification

All payloads include an `X-Webhook-Signature` header when a secret is configured. The signature is computed as:

```
sha256=HMAC-SHA256(payload_body, secret)
```

To verify on the receiving end:

```go
import "github.com/ogulcanaydogan/llm-slo-ebpf-toolkit/pkg/webhook"

valid := webhook.VerifyHMAC(requestBody, secret, signatureHeader)
```

## Retry Behavior

- Up to 3 delivery attempts with exponential backoff (1s, 2s, 4s).
- 5xx responses trigger retries.
- 4xx responses are not retried (considered permanent failures).
- Network errors trigger retries.
