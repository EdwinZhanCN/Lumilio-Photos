# Decision: Publish one immutable CalVer release across Desktop and Server

Status: implemented

## Problem

Desktop packages and the multi-architecture Server image were published by two
independent workflows. A tag could therefore leave a GitHub Release available
while the Server image failed, or publish an image without the checksums and
deployment files that identify the matching application build. Version stamping
also disagreed across surfaces: Docker retained the Git tag's `v`, Desktop
removed it, macOS wrote a pre-release label into numeric bundle fields, and the
Windows executable retained the placeholder `0.1.0` metadata.

The default Compose files intentionally remain convenient development and
evaluation entry points, but their mutable `latest` fallback is not an
auditable production upgrade boundary.

## Decision

Lumilio Photos uses the SemVer-compatible CalVer form
`YY.TRAIN.PATCH[-beta.N|-rc.N]`. Git tags add the conventional `v`; product
metadata, API health, Desktop state, and OCI labels omit it. The first release
in this scheme is `v26.1.0-beta.1`.

Tag pushes run one release workflow. It accepts only the fixed CalVer grammar,
requires the tagged commit to belong to `main`, builds the shared Web SPA,
macOS and Windows Desktop packages, and native amd64 and arm64 Server images,
then creates the GitHub Release after every delivery artifact succeeds. The
release includes SHA-256 checksums, a machine-readable manifest, and a Server
bundle whose `.env` pins the multi-architecture image by OCI digest. Manual
dispatch remains a non-release path that updates `edge` and retains Actions
artifacts without creating a GitHub Release.

Product and platform versions are separate build inputs. Desktop embeds the
full product version such as `26.1.0-beta.1`; macOS uses the numeric
`26.1.0` marketing version plus a monotonic workflow build number, and Windows
uses the numeric core for fixed file metadata while retaining the full product
version as descriptive metadata and installer display text.

`beta.N` remains the production-test channel while UI and business behavior
can change. `rc.N` begins only after feature freeze. Stable fixes increment
`PATCH`; the next feature train increments `TRAIN`. Database, configuration,
backup, API, and Lumen compatibility remain owned by their independent typed
schema and lock versions. Rolling back a forward-migrated catalog pairs the
previous binary or image with its pre-upgrade database snapshot.

## Alternatives considered

### Use `26.1-b1`

Rejected because it is not a complete SemVer version and does not work with
the repository's SemVer-aware Docker metadata path. A dot-separated numeric
pre-release identifier also preserves numeric ordering after beta 9.

### Keep separate Desktop and Docker release workflows

Rejected because separate terminal jobs cannot make the public release appear
only after both delivery families succeed. Coordination through an existing
GitHub Release would preserve a visible partial state and add recovery logic.

### Make the checked-in Compose default a concrete release version

Rejected because every release would create a mechanical source change and a
checkout of another branch could silently retain an unrelated image pin. The
release-owned Server bundle is immutable and records the resolved OCI digest;
the source Compose fallback remains useful for evaluation and explicit edge
work.

### Publish every successful `dev` commit

Rejected because a production-test artifact must identify a reviewed release
commit and have a durable rollback target. `dev` remains the integration
branch, `main` remains the release branch, and manual workflow runs provide
disposable edge builds when a tag is unnecessary.

## macOS icon packaging

The macOS icon source is `desktop/build/appicon.icon`. Release builders require
macOS 26+ and Xcode 26+ because the pinned Wails Icon Composer generator checks
both. The generator receives an explicitly empty PNG input, so unsupported
toolchains fail instead of falling back to `appicon.png`. Both `Assets.car` and
the compatibility ICNS generated from the same Composer source are bundled.
Missing `Assets.car` fails packaging. Windows continues using its PNG source.
The beta.1 build used macOS 15 and silently took Wails' PNG fallback; successful
compilation alone therefore did not establish the intended icon source.
