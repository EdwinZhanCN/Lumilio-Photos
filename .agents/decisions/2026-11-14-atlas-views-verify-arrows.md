# Decision: Atlas views verify their arrows; anchor drift is a notice

Status: implemented, 2026-11-14 (#273, A4 of the
[Atlas rules plan](../../docs/exec-plans/active/atlas-rules.md)). Supersedes
the "hard-fail on every anchored-code change" part of the
[anchored code map decision](2026-10-01-atlas-anchored-code-map.md); the rest
of that record stands.

## Problem

The 2026-10-01 gate failed CI whenever a `go:` or `ts:` anchor's source
changed, until someone re-read the view and ran `task atlas:lock`. Two things
were wrong with that. It checked the wrong fact: a hash says the code
changed, not that the drawing is now false, so every edit that touched an
anchor cost a lock and the lock became routine. And it could not catch the
drift that mattered: when A4 first checked the seven sequence and data-flow
views against the typed facts, they drew calls the code no longer made
(`SearchAssets` and `HydrateAssets` where the handler and service call
`SearchBrowseItems` and `browseItemsForMediaRefs`), a worker reaching the
scanner directly where it goes through `server/app`'s pipeline runtime, and
River and the LLM provider as code participants. All of them had passed the
hash gate because someone had locked them. Phase B of the server-structure
plan moves most of the Server's packages, so it would rewrite nearly every
anchor and force a re-lock of every view on every PR.

## Decision

- **Arrows are facts, checked as errors.** Every sequence message and
  data-flow edge whose ends are both `go:` or Go `mod:` anchors must exist in
  the type-resolved code. Each hop X → Y holds when a body of X refers to Y
  directly, through an interface Y implements, through a module function that
  returns Y, or through a func value the module stores in a field or passes
  as an argument (constructor, setter, or wrapper injection). Declared
  `via:` hops are checked one by one. A step anchor on the callee end narrows
  the last hop to that method; on neither end, a type is a message both ends
  must name and a function is a wiring site that must refer to both. Replies
  run backwards; data-flow edges hold either way. A missing arrow fails
  `atlas check` and suggests the shortest chain the code has (six calls at
  most).
- **Anchor drift is a notice.** A changed hash prints the view and
  `file:line` and leaves the exit code alone. The lock still records what was
  verified, and the skill requires handling every notice in the same PR.
- **Generated pages link files, not lines**, so a refactor that keeps every
  drawn call changes nothing checked in. The site data keeps exact lines.
- **Refactor evidence is a tool, not a review chore.** `atlas split-report`
  gives a package's type-resolved file graph and, for proposed clusters, the
  crossing edges, unexported symbols, and cycles. `atlas movecheck --base`
  pairs functions across trees by receiver and name with qualifiers and
  export case folded, and lists changed bodies for a one-line reason each.

Validation: removing `ApplyIngestReceipt` from `pipelineRuntime.ingest`
fails `upload-to-catalog`, and a comment added to the same function produces
one notice and exit 0. `testdata/arrows/{pass,violation}` covers every hop
kind; eight mutations of the pass fixture (setter not installed, setter
storing nothing, wrapper dropping its argument, field filled with something
else, a call removed, an interface call removed, a message no longer named,
a package no longer used) each fail it.

## Alternatives considered

- **Keep hard-failing on hash drift.** Kept the cost and missed the false
  arrows above; during Phase B it would make every PR a re-lock of every
  view, which trains people to lock without reading, the failure the gate
  exists to prevent.
- **Drop the hash lock entirely.** Arrow checks cover calls but not labels,
  notes, order, or conditions. The notice keeps that semantic drift visible
  at no CI cost.
- **Verify arrows by name or text match** (the callee's name appears in the
  caller's source). Breaks on interfaces, injected func values, and renamed
  imports, and passes on comments; it would repeat the textual-check mistake
  `architecturecheck` was deleted for.
- **A full call graph (`go/callgraph` with pointer or VTA analysis).** More
  precise for func values, but slow on the whole module, and it answers
  "can this ever run", which is wider than "does this body name that
  participant". The direct-reference rule plus declared `via:` hops keeps
  each arrow's evidence readable from the YAML.
- **Allow arrows to unverified participants silently.** `ts:`, `api:`,
  `sql:`, and `ext:` ends stay unchecked, but `via:` on them is an error, so
  an unchecked arrow never looks checked.
