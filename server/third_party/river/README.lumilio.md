# Lumilio River production fork

This directory contains the production Go files reachable from
`github.com/riverqueue/river` v0.24.0 under its original MPL-2.0 license.

The fork excludes upstream dependency tests. River's tests are PostgreSQL-based,
and Go module graph pruning otherwise retains those drivers even in a
SQLite-only application. Production behavior is unchanged except that the
missing-driver diagnostic is backend-neutral.

The companion `third_party/river-rivershared` replacement removes the remaining
production-to-test-helper import. Keep both replacements pinned to the River
version declared by the Server and Desktop modules.
