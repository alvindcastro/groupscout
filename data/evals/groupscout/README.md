# GroupScout Eval Fixtures

These JSONL files are the golden fixture set for AI quality work. GQ1 defines the cases, and GQ2 loads them through `internal/evalops`.

| File | Cases | Coverage |
|---|---:|---|
| `lead_cases.jsonl` | 12 | Construction, permits, Creative BC, VCC, Eventbrite, BCBid, and infrastructure announcements. |
| `alert_cases.jsonl` | 5 | YVR delay spike, weak single-signal noise, severe multi-signal disruption, and stale data fail-closed behavior. |
| `adversarial_cases.jsonl` | 5 | Prompt injection, raw PII, conflicting dates, suspicious contact data, and secret-like text redaction. |

Total: 22 synthetic cases.

The schema is documented in `docs/evals/groupscout-case-schema.md`.

Fixture rules:

- Do not replace synthetic text with live website snapshots.
- Do not add real customer, traveler, passenger, student, sales, or private portal data.
- Keep case IDs stable because future reports and changelogs will cite them.
- Add a changelog entry whenever cases are added, removed, renamed, or materially re-scoped.

GQ5 draft cases generated from review samples are not golden fixtures yet. Keep draft JSONL outside this directory until a human reviewer replaces all `TODO_REVIEW_*` fields, confirms the synthetic source text is safe, chooses whether the case is release-blocking, updates this README's counts, and records the promotion in `docs/CHANGELOG.md`.
