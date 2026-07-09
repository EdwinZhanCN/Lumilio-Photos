# Bundled license texts

Full license texts for Lumilio Photos and the native components the desktop
build bundles. They are embedded into the desktop binary (`desktop/licenses.go`)
so the first-run onboarding window can display them, and the build scripts stage
this directory into the bundle (`Resources/licenses` on macOS,
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
license). The component ↔ license mapping lives in `desktop/licenses.go`
(`licenseManifest`); update both when the set of bundled components changes.
