# Duplicates, Likes & Trash

## Duplicates

Duplicate detection groups candidates inside one Repository into exact
duplicates, visually similar files, or mixed groups. An administrator starts
it from the **Storage** page (**Detect duplicates**); you then pick the file to
keep, merge, or mark a group as "not a duplicate".

Merging combines albums and tags onto the kept file, keeps the higher rating,
keeps a like if either file had one, and keeps the kept file's description
(or the duplicate's when the kept one is empty). Faces are not moved unless
you ask for it, and only in an exact-duplicate group. The duplicates you do
not keep are moved to the Trash; the space they use is released when they
expire or you delete them permanently.

## Trash

**Move to Trash** moves the file out of your folder into that Repository's own
Trash (its private `.lumilio/trash` directory); it is not the operating
system's trash. **Restore** puts the file back at its original path with its
albums, rating, like, tags, description, and People intact.

- Administrators open the Trash per Repository from the **Storage** page;
  every user sees their own items under **Collections → Trash**.
- Trashed files are kept for 30 days and then deleted permanently. In Docker
  the period is `repository_trash.retention_days` in `server.toml`; Desktop
  uses 30 days. **Delete permanently** removes a file now.
- A database backup made before a deletion lists the deleted file in the
  Trash after it is restored, instead of reporting it missing.

### Missing files

When a file disappears outside Lumilio (an unplugged drive, a deletion in
Finder or Explorer), the photo leaves browsing, search, albums, and the
Storage counts, and the Repository's **Missing** view on the Storage page
lists it. When the same file comes back at its path, the photo returns with
its metadata. **Remove missing items** forgets missing files without touching
the disk; a photo left with no file at all loses its metadata too.

Rewriting a file in place in another app, including a metadata-only XMP or
EXIF write, does not make it missing: the photo keeps its albums, rating,
like, tags, description, and People, and its thumbnails are regenerated.

::: warning The Trash is not a backup
The Trash is for undoing mistakes. RAID, sync folders, and cloud drives each
cover only part of the risk; none replaces an independent, restore-verified
backup (see [Backup and Data Integrity](../introduction/integrity)).
:::

## Smart Classifiers

<!-- This section is a placeholder for the English version. -->
