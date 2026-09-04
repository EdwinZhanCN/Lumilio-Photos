# Pinned Lumen Hub release (`lumen.lock.json`)

Photos embeds one immutable Lumen Hub release for its managed Desktop path.
`lumen.lock.json` is the dependency boundary; it does not attempt to mirror the
complete Hub product model.

## Owned files

- `lumen.lock.json` — schema version 2. `release` is the selected Hub tag;
  `revision`, manifest hash, generated catalog hash, and vendored control-proto
  hash are derived by reconciliation.
- `desktop/internal/lumen/release_catalog.go` — generated Desktop artifact
  records and the small official preset ID list published by that release.
- `desktop/internal/lumen/controlv1/control.proto` — vendored byte-for-byte from
  the pinned Hub revision.

The generated catalog contains only facts required to install and launch the
pinned binary. Models, datasets, service graphs, backend runtime strings, and
Hub YAML semantics remain owned by Hub.

## Deterministic and remote commands

Pin, reconcile, and verify:
[lumilio-pin-reconcile](../.agents/skills/lumilio-pin-reconcile/SKILL.md).

```text
task lumen:check                  # committed local state only; no network
task lumen:verify                 # prove the current pin against GitHub
task lumen:sync                   # remotely verify and reconcile current tag
task lumen:sync RELEASE=vX.Y.Z    # select and reconcile a new Hub release
```

`lumen:check` verifies the generated catalog hash and release, the vendored
proto hash, and the Compose preset references. It is safe for every CI run and
cannot fail because GitHub or its release CDN is unavailable.

`lumen:verify` and `lumen:sync` additionally require:

1. a schema-version-2 `manifest.json` whose version matches the tag;
2. a checksum file that explicitly covers `manifest.json` and every referenced
   artifact;
3. all Desktop profiles and all Compose preset IDs;
4. a tag revision matching the lock; and
5. a control proto matching the pinned Hub revision.

`sync` writes the lock and generated catalog atomically after all remote checks
succeed. Renovate may propose a release tag, but a maintainer performs this
reconciliation and reviews the complete generated diff.

## Runtime ownership

Desktop persists only setup intent: preset, download region, and model-cache
directory. Before a managed start it invokes the exact installed binary:

```text
lumen-hub config render --target desktop ...
```

That binary uses `lumen-schema`, validates the result, and writes the derived
configuration. Desktop never reconstructs or validates Hub YAML itself.

Site documentation is explanatory content, not a machine consumer of this
catalog. Descriptive copy may evolve independently so long as it does not
become a second runtime configuration implementation.

## Upgrade order

1. Apply and publish the Hub change under a new immutable tag.
2. In Photos, run `task lumen:sync RELEASE=<new-tag>`.
3. Review and commit `lumen.lock.json`, the generated release catalog, and any
   vendored proto change together.
4. Run `task lumen:check` on every normal change and `task lumen:verify` at the
   dependency/release boundary.
