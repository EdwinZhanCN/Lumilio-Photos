# Decision: Use decoded image contracts instead of loader guesses

Status: implemented

## Problem

The beta.1 native library selected Ultra HDR for a JPEG that the local library
loaded as ordinary JPEG. Application magic-byte detection passed a JPEG-only
loader autorotate option and made thumbnail generation fail. Related audit
findings included inconsistent orientation, preview validity inferred from an
EOI offset, and helpers returning success after skipping failed encodes.

## Decision

Thumbnail orientation belongs to libvips thumbnail_buffer. Full-image display
operations use metadata-based autorotation on the decoded image; format magic
bytes never establish loader option support. RAW container rotation remains
explicit and separate. Preview acceptance must evaluate pixels, and encoding
helpers must propagate failures and honor their declared output format.
Native startup failures propagate to app.Run.

Upload precheck uses required JSON membership sets: an empty full or quick
collection is [], not the optional-filter helper's nil converted to an empty
string. Candidate matches never imply verified duplicates.

The complete call inventory, regressions, native probe and remaining coverage
boundaries are in docs/govips-audit.md.

## Alternatives considered

- Pinning an older libvips build hides an application assumption and does not
  establish safety when codec selection changes again.
- Special-casing Ultra HDR headers duplicates native decoder selection and
  repeats the same maintenance problem for the next format.
- Retrying without arbitrary options masks the contract error and adds a
  costly failed decode to every affected input.
- Treating readable image headers or visible gallery cards as successful
  processing cannot prove usable pixels or detect silent partial results.
