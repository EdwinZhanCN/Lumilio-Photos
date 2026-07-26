# Lumilio River shared patch

This directory contains the production Go files used from
`github.com/riverqueue/river/rivershared` v0.24.0 under its original MPL-2.0
license.

The only behavior change is in `testsignal`: its test-only wait helper uses the
same upstream default timeout directly instead of importing
`riversharedtest`. Upstream's production `testsignal` package imports that
PostgreSQL test-helper package, which otherwise links pgx into SQLite-only
Lumilio binaries.

Keep this replacement pinned to the River version in the Server and Desktop
modules. Remove it when upstream makes the production test signal independent
of PostgreSQL test infrastructure.
