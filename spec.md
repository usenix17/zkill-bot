# ZKill Bot System Specification

## 1. Purpose

ZKill Bot is an automated EVE Online monitoring system with one job:

1. Continuously ingest new killmail events from the zKillboard R2Z2 stream and run configurable rule-based actions.

It is designed for always-on operation and operational visibility.

## 2. Product Outcomes

The system is expected to:

- Detect and process new killmail events in near real time.
- Enrich killmail data so rules can be based on higher-level game context.
- Evaluate flexible, prioritized rules and trigger one or more actions per match.
- Prevent duplicate action execution for the same event/action combination.

## 3. High-Level Functional Scope

The zkill stream is described in r2z2-docs.md.

### 3.1 Killmail Event Pipeline

The killmail polling system:

- Starts from either a stored checkpoint or a live sequence source.
- Polls the event feed continuously.
- Handles normal flow (new killmail), throttling, and transient errors.
- Normalizes incoming payloads into a consistent internal event shape.
- Rejects malformed payloads and moves forward without blocking the pipeline.
- Enriches event data with ship/item names, meta levels, and related type metadata.
- Evaluates configured rules in deterministic order.
- Executes configured actions for matched rules.
- Records processing progress and operational counters.

### 3.2 Rule System

Rules are externally configurable and support:

- Enable/disable per rule.
- Priority ordering.
- Two evaluation modes:
  - First-match mode (stop after first matched rule).
  - Multi-match mode (continue evaluating remaining rules).
- Composable filter logic (`and` / `or` / `not`) and domain-specific leaf filters.

Supported filter families include:

- Attacker volume and solo-kill logic.
- Region/system/day/time-window filters.
- Character/corp/alliance filters (victim-side and attacker-side variants).
- Ship and item-type matching.
- Capital involvement detection.
- zKillboard value thresholds.

Rules should be written in a human readable format.

## 4. Action Features

Actions are rule-driven outputs. Current built-in actions are:

- Console output action for human-readable event inspection.
- Webhook action for outbound integrations (including Discord-friendly summaries).

Action behavior requirements:

- Action arguments can be provided per rule (where supported).
- Each action execution is idempotency-protected.
- Failed retryable actions are retried automatically up to configured limits.

## 5. Reliability and Idempotency

The system provides operational safety features:

- Sequence progress and processed state are persisted.
- Action history prevents duplicate execution for the same event/action fingerprint.

## 6. Observability and Operator Alerts

The system emits structured human readable logs and runtime metrics when a DEBUG=true flag is used, including:

- Fetch status distributions.
- Processed/rejected killmail counters.
- Rule match counts.
- Action success/failure/retry/skip/unknown counters.
- Processing latency and ingestion lag.

There is also startup/shutdown notifications to a Discord webhook

## 7. Configuration and Runtime Controls

The system is environment-configurable for:

- Rule file location.
- Discord webhooks for startup/shutdown notifications

Configuration is validated at startup. Invalid configuration fails fast with actionable errors.

## 8. Data Persistence Expectations

Persistent state includes:

- Last processed sequence checkpoint.

Persistence is intended to preserve continuity across restarts and support safe recovery workflows.

## 12. Acceptance Criteria (Feature-Level)

A deployment is considered functionally correct when:

- It can run continuously without manual intervention.
- New killmails are consumed, normalized, enriched, evaluated, and actioned.
- Duplicate action execution is prevented across retries/restarts.
