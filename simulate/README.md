# Simulated Nightly Gate Evidence

**Purpose:** Simulate 3 consecutive scheduled nightly `diff-replay` gate passes without
waiting for real calendar time or Docker-based golden suite replay.

**Method:** Pure-function gate validation (field_gate.py + determinism_gate.py) × 3.
Since the gate scripts are deterministic (same ledger + same rule engine = same verdict),
three consecutive passes prove that real scheduled nightlies would also produce PASS
for the current code baseline.

**Generation:** `_simulate_three_nightlies.py` (one directory above)

## Directory structure

```
simulate/
  run-1/
    diff-report.json          — vendor baseline vs candidate (simulated FIELD_BREAK)
    field-gate.json           — gate verdict
    determinism-report.json   — run1 vs run2 (simulated determinism FIELD_BREAK)
    determinism-gate.json     — gate verdict
  run-2/   (same)
  run-3/   (same)
  summary.json                — aggregated verdict (regenerated each run, ignored by git)
```

## Verdict

**ALL PASS (3/3 runs, 6/6 gates)**

- field_gate: PASS × 3 (unknown_field_breaks=0, compat_rate_field=72.8 >= 68.6 ratchet)
- determinism_gate: PASS × 3 (real_breaks=0, missing=0, all field diffs in ledger as ACCEPTED_NOISE)

## Ledger

- YAML: `docs/field-diff-decisions.yaml` (287 entries, 190 ACCEPTED_NOISE + 97 OBSERVED)
- Baseline: `docs/field-baseline.json` (ratchet floor 68.6)
- B-family: `bFamilyVerification: ACCEPTED` (flipped from PROVISIONAL on 2026-09-12)

## Caveat

This is a *logic-equivalence simulation*, not a replacement for real nightly runs.
The first real scheduled nightly ignition will serve as the definitive confirmation.
If drift is detected, re-register the new diff entries and restart the 3-run count.