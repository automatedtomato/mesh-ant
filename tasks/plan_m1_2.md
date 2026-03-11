# Implementation Plan: M1.2 — Example Trace Dataset

**Branch:** `feat/m1-trace-dataset` (cut from develop)
**Status:** Confirmed — ready to implement

## Scenario

A vendor registration application moving through a regional government procurement office:
submitted → form validation → rate-limited → classified → approval threshold exceeded →
redirected to compliance sub-queue → 72-hour background check → approved.

Chosen because: non-human mediators (rules, forms, policies, routing matrices) have natural
starring roles; delays and thresholds are structural; attribution gaps are realistic.

## File to Create

`data/examples/traces.json` — 10 trace objects, valid against meshant/schema/trace.go

## 10 Traces (chronological)

| # | what_changed | tags | source | target | mediation | observer |
|---|---|---|---|---|---|---|
| 1 | Application received by intake queue | — | [vendor-web-portal, vendor-registration-form-v4] | [intake-queue] | — | portal-ingestion-log/position-intake |
| 2 | Held in pending-correction (missing DUNS) | blockage | [intake-form-validator-v4] | [vendor-registration-application-00142] | intake-form-validator-v4 | form-validator-service/process-log |
| 3 | Re-entered queue after vendor correction | — | nil | [intake-queue] | — | portal-ingestion-log/position-intake |
| 4 | Buffered 38 min by rate-limiter (>50/hr) | delay, threshold | [rate-limiter, queue-throughput-policy-v2] | [vendor-registration-application-00142] | queue-throughput-policy-v2 | queue-monitor/ops-dashboard |
| 5 | Classified as Tier 2 Regional Vendor | translation | [classification-ruleset-v7] | [vendor-registration-application-00142] | classification-ruleset-v7 | classification-service/event-log |
| 6 | $340k exceeds $250k ceiling → escalation | threshold, redirection | [approval-threshold-rule-tier2] | [vendor-registration-application-00142] | approval-threshold-rule-tier2 | approval-engine/threshold-monitor |
| 7 | Redirected to compliance sub-queue | redirection | [escalation-routing-matrix-v3] | [vendor-registration-application-00142] | escalation-routing-matrix-v3 | routing-engine/event-stream |
| 8 | 72-hour wait: background check in progress | delay | [background-check-service] | [vendor-registration-application-00142] | — | compliance-queue/wait-monitor |
| 9 | Background check result received via webhook | translation | nil | [vendor-registration-application-00142] | background-check-webhook-endpoint | compliance-queue/inbound-webhook |
| 10 | Application approved by compliance reviewer | threshold | [compliance-reviewer, compliance-approval-checklist-v2] | [vendor-registration-application-00142] | compliance-approval-checklist-v2 | compliance-review-ui/audit-log |

## Requirements Coverage

| Requirement | Traces |
|---|---|
| delay | #4 (38 min rate-limit buffer), #8 (72 hr background check) |
| threshold | #4 (queue capacity), #6 (dollar ceiling), #10 (final approval) |
| redirection | #6 (triggers escalation), #7 (routes to compliance sub-queue) |
| blockage | #2 (missing DUNS field holds application) |
| non-human mediator | #2, #4, #5, #6, #7, #9, #10 |
| multiple sources | #1 (vendor+form), #4 (rate-limiter+policy), #10 (reviewer+checklist) |
| absent source | #3 (automated resubmission), #9 (webhook with no system id) |
| observer variation | 8 distinct observer positions |

## UUID List (pre-assigned for dataset)

```
b3d7e1a2-4f09-4c8e-9a1b-c2d3e4f5a6b7  (trace 1)
c4e8f2b3-5a10-4d9f-ab2c-d3e4f5a6b7c8  (trace 2)
d5f9a3c4-6b21-4eaf-bc3d-e4f5a6b7c8d9  (trace 3)
e6a0b4d5-7c32-4fbf-cd4e-f5a6b7c8d9e0  (trace 4)
f7b1c5e6-8d43-4acf-de5f-a6b7c8d9e0f1  (trace 5)
a8c2d6f7-9e54-4bdf-ef6a-b7c8d9e0f1a2  (trace 6)
b9d3e7a8-af65-4cef-f07b-c8d9e0f1a2b3  (trace 7)
c0e4f8b9-b076-4def-017c-d9e0f1a2b3c4  (trace 8)
d1f5a9c0-c187-4eff-128d-e0f1a2b3c4d5  (trace 9)
e2a6b0d1-d298-4fff-239e-f1a2b3c4d5e6  (trace 10)
```

## Timestamps

Traces 1–8 span 2026-03-10T09:14Z to 2026-03-10T11:27Z (~2 hrs processing day 1).
Traces 9–10 are 2026-03-13 (72 hours later, after background check completes).

## Implementation Steps

1. `mkdir -p data/examples`
2. Write `data/examples/traces.json` from plan above
3. Validate each trace manually against schema field rules
4. Optionally: write a small Go validation script or test that loads the file and calls Validate() on each record
5. Commit on `feat/m1-trace-dataset`
