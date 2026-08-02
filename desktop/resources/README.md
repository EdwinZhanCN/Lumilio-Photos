# Desktop resources

`manifest.json` is the generated resource contract. Release builds populate
its platform/architecture entries with content-addressed media tools and
licenses; the Desktop host verifies every entry before materialising a new
version directory. Missing tools are a recovery condition, never a silent
`PATH` fallback.
