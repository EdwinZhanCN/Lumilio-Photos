# Vendored SQLite Vec1

- Upstream: `https://sqlite.org/vec1`
- Release: `version-0.7`
- Source URL:
  `https://sqlite.org/vec1/raw/vec1.c?ci=version-0.7`
- `vec1.c` SHA-256:
  `8571bb4f77f9547d11ad11e2f72e0de7d3b2ab44e7930151998bce9377ed4b86`

The source is public domain. The x86 build includes scalar and AVX2/FMA
translation units and selects AVX2 at runtime using Vec1's upstream dispatch.
Arm64 uses the source's native NEON path.

To update the vendored source, download a named upstream release, verify and
record its SHA-256 here, then run the full Server and Desktop native CGo gates.
