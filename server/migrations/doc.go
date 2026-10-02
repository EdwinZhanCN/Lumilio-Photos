// Package migrations embeds the catalog baseline
// (000001_storage_baseline.up.sql, schema version 1, the v26.1.0-rc.1
// compatibility baseline) and its numbered forward steps, so they apply
// without depending on the working directory — the Desktop bundle has no
// repository checkout and an unpredictable CWD.
//
// The baseline is edited in place only until the rc.1 tag. After it, schema
// changes are numbered steps under steps/ (contributor rules in its README.md)
// and shipped files are never edited. The decision is
// .agents/decisions/2026-09-24-rc-compatibility-baseline.md. QueueDB River
// migrations are independent and disposable.
//
//atlas:group catalog
package migrations
