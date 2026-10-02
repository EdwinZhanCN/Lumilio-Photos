// Package cloud provides the cloud storage abstraction layer for importing
// remote assets into a local Lumilio repository (Cloud → Local sync).
//
// Layering:
//
//	CloudProvider (iCloud today; other providers may implement the interface)  ← interface
//	      ↓
//	SyncStateStore                     ← pagination cursor + etag dedup
//	      ↓
//	CloudImportSource                  ← implements sourcing.AssetSource
//	      ↓
//	CloudSyncConsumer                  ← acknowledged materializer loop
//
//atlas:group ingest
package cloud
