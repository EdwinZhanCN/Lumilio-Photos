# iCloud Go Client

A native Go iCloud client for listing and downloading photos from Apple iCloud Photos, adapted for use within the Lumilio cloud sync layer.

## Acknowledgments

This module is derived from **[Moon3r/icloudgo](https://github.com/Moon3r/icloudgo)** (MIT License), which in turn is built upon **[chyroc/icloudgo](https://github.com/chyroc/icloudgo)** (Apache License 2.0), the original Go iCloud API client.

We are grateful to the authors and contributors of both projects for their work in reverse-engineering the iCloud protocol and making it accessible to the Go ecosystem.

## Modifications

The following changes were made when adapting the upstream code:

- Flattened the `internal/` sub-package into a single `icloud` package.
- Removed unused services: Drive, photo upload, photo delete, folder management.
- Stripped album support to `All Photos` only; removed user-created folder traversal.
- Added accessor methods: `Fingerprint()`, `MIMEType()`, `IsDeleted()` on `PhotoAsset`, and `SetTwoFACodeGetter()` on `Client`.
- Removed dependencies on SQLite, BadgerDB, CLI framework, and the Synology DSM toolchain.

## License

This module retains the license of its origin. See the [LICENSE](./LICENSE) file for the full terms of the Apache License 2.0.
