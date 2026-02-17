# Contracts v1 (LLM SLO eBPF Toolkit)

Versioned public interface contracts for SLO telemetry and incident attribution.

## Schemas
- `slo-event.schema.json`: normalized SLI/SLO event envelope.
- `incident-attribution.schema.json`: root-cause attribution output envelope.

## Compatibility Policy
- Minor, backward-compatible additions are allowed within `v1`.
- Breaking changes require a new version folder (`v2`).
- Contract changes must include benchmark parser fixture updates.
