// Package migrations embeds the SQL catalog baseline and its forward steps so
// they can be applied without depending on the working directory or on the
// files being present on disk. This is required for the desktop bundle (which
// has no repo checkout and an unpredictable CWD) and also makes docker/dev
// startup CWD-independent.
//
//atlas:group catalog
package migrations
