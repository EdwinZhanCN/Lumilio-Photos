# Bundled license texts

Full license texts for Lumilio Photos and the native components the desktop
build bundles, plus the generated `THIRD_PARTY_NOTICES.txt` covering runtime Go
modules and npm packages. The application license and notices are embedded into
the desktop binary for the legal links, and the build scripts stage this
directory into the bundle (`Resources/licenses` on macOS,
`resources\licenses` on Windows) so the texts also ship as plain files.

Provenance: canonical SPDX texts, fetched with

```sh
base=https://raw.githubusercontent.com/spdx/license-list-data/main/text
curl -fsSL -o PostgreSQL.txt        $base/PostgreSQL.txt
curl -fsSL -o LGPL-2.1.txt          $base/LGPL-2.1-only.txt
curl -fsSL -o GPL-2.0.txt           $base/GPL-2.0-only.txt
curl -fsSL -o Artistic-1.0-Perl.txt $base/Artistic-1.0-Perl.txt
curl -fsSL -o MIT-Wails.txt https://raw.githubusercontent.com/wailsapp/wails/master/LICENSE
```

`GPL-3.0.txt` is a copy of the repository root `LICENSE` (the app's own
license). Regenerate dependency notices after dependency changes:

```sh
cd web && vp install
cd .. && node desktop/scripts/generate-third-party-notices.mjs
```

The generator inventories runtime Go packages from `desktop/` and `server/`,
then installed packages from `web/node_modules`, preserving package license and
NOTICE files in one distributable artifact.
