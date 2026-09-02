# Postmortems

Escaped bugs: a defect reached a user, `main`, or a release, and the
interesting part is why every safety net missed it. Format and the
guardrail-linking rule are owned by
[agent-harness.md](../../docs/agent-harness.md#postmortems).

Files are `NNNN-slug.md`, sequentially numbered. `## Guardrails added` must
link the real merged artifacts (regression test, CI gate, rule); a postmortem
without a merged guardrail is not finished. An empty directory is a good
sign, not a gap.
