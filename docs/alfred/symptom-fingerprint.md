# Symptom Fingerprint Vocabulary

A **fingerprint** is the load-bearing primitive for the symptom bus. Every downstream
behaviour — dedup, coalescing, noise rules, circuit-breaking, playbook matching — keys
on it. An unstable fingerprint breaks all of them.

## Format

```
<dimension>:<value>(|<dimension>:<value>)*
```

- Dimensions are sorted **alphabetically** before joining. This makes the fingerprint
  deterministic regardless of the order emitters discover facts.
- Values are URL-encoded if they contain `|` or `:`.
- The fingerprint is used as the Kafka message **key** on `forge.symptoms.v1`.

### Example

```
error_class:ECONNREFUSED|port:8094|service:workflow-registry|signal:probe-failed
```

## Required Dimensions

| Dimension | Type   | Description                                              |
|-----------|--------|----------------------------------------------------------|
| `service` | string | Name of the affected service as registered in the runtime registry. |
| `signal`  | string | Signal class (see enum in schema). One of: `probe-failed`, `probe-recovered`, `log-pattern`, `metric-threshold`, `ci-failure`, `webhook`. |

## Optional Dimensions

| Dimension     | Type    | Description                                                  |
|---------------|---------|--------------------------------------------------------------|
| `tenant`      | uuid    | Tenant UUID when the symptom is tenant-scoped.               |
| `workspace`   | uuid    | Workspace UUID when the symptom is workspace-scoped.         |
| `error_class` | string  | Machine-readable error class (e.g. `ECONNREFUSED`, `OOMKilled`, `CrashLoopBackOff`). |
| `port`        | integer | Network port relevant to the symptom (e.g. probe target port). |
| `route`       | string  | HTTP route relevant to the symptom (normalised, no query params). |

Unknown dimensions MUST NOT be included; the triager rejects events with unknown
dimensions to the DLQ.

## Canonicalisation Rules

1. Sort all `dimension:value` pairs by dimension name (ASCII order).
2. Join with `|`.
3. No trailing `|`.
4. Dimensions with empty values are omitted entirely.
5. Values are trimmed of leading/trailing whitespace.
6. `port` values are stringified integers with no leading zeros.

## Stability Contract

Emitters MUST NOT change the dimensions or values of a fingerprint for a given class
of symptom. Fingerprint changes break noise rules and circuit breakers that reference
the old fingerprint. When a new dimension is relevant, add it as optional (it produces
a new fingerprint naturally).

If a breaking change is unavoidable, ship a migration that:
1. Adds the new fingerprint to any noise rules that matched the old one.
2. Resets the circuit-breaker state for the old fingerprint.
3. Documents the change in a `CHANGELOG` entry under `contracts/events/`.

## Triager Validation

The triager validates fingerprints on ingestion:

- All `required` dimensions present.
- All dimensions in the enumerated vocabulary (`service`, `signal`, `tenant`,
  `workspace`, `error_class`, `port`, `route`).
- Dimensions sorted alphabetically.
- No duplicate dimensions.

Events failing validation are routed to `forge.symptoms.v1.dlq` with a
`validation_error` header describing the failure.
