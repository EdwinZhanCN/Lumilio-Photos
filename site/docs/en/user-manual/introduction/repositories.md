# Storage Locations and Repositories

::: warning Beta software
Start with test media or a library that already has a reliable backup. Do not use Lumilio Photos as the only copy of important media.
:::

Lumilio separates a directory that is **authorized to contain repositories** from a **repository that stores media**:

- A **Storage Location** is an authorized parent directory. Its portable identity is stored in `.lumilioroot`.
- A **repository** is an independent directory containing media and repository configuration. Its identity is stored in `.lumiliorepo`.

One Storage Location can contain multiple repositories. A directory cannot be both a Storage Location and a repository.

```text
Storage Location/
├─ .lumilioroot
├─ primary/
│  ├─ .lumiliorepo
│  └─ .lumilio/
└─ Family Photos/
   ├─ .lumiliorepo
   └─ .lumilio/
```

## Three operations that look similar

| Operation | Where | Directory to select | Initializes a repository? |
| --- | --- | --- | --- |
| **Add location** | Desktop Control Panel | An existing parent directory, such as `Lumilio/` on an external drive | No; it only registers or creates `.lumilioroot` |
| **Create repository** | Repository creation in the browser | Select a registered, active Storage Location | Yes; it creates a child directory and `.lumiliorepo` |
| **Attach repository** | Desktop Control Panel | An existing repository directory containing `.lumiliorepo` | No; it only registers the existing repository |

::: tip Create a new repository on an external drive
First use **Add location** in the Desktop Control Panel and select the parent directory on the drive. Then create the repository in the browser and select that Storage Location. Do not give an empty directory to **Attach repository**; attachment is only for an existing repository.
:::

## Adding a Storage Location does not create a repository

**Add location** authorizes a directory as a repository container. Desktop creates `.lumilioroot` when the directory does not already have one. An existing marker identifies the same location after a mount path or drive letter changes.

After registration, the location appears in the browser repository form. Creation lets you select:

- the Storage Location;
- file layout: capture date, flat, or content-addressed;
- filename-conflict handling: rename, unique ID, or overwrite;
- an optional cloud-source credential.

Local and cloud-backed creation use the same location and file policies. Cloud creation only adds the cloud credential.

The primary repository is the exception: first-run setup creates it in the non-removable default Storage Location. Regular repositories can use any registered, active Storage Location.

## Attaching is only for an existing repository

**Attach repository** registers a Lumilio repository that already exists on disk. The selected directory must contain a valid `.lumiliorepo`; an empty directory or an ordinary folder of photos is not initialized automatically.

If that repository identity is already registered at another path, Desktop asks you to decide explicitly:

- **Use as moved location** when the repository moved to another disk, mount point, or Windows drive letter.
- **Register as copy** when this directory is an independent copy that needs a new identity.

Lumilio does not guess whether a directory moved or was copied based on whether its old path is currently online.

## Offline and moved locations

When an external drive or network volume disconnects, its Storage Location and repositories become offline. Lumilio preserves their identity and browsing records, but refuses writes until they reconnect. It does not silently create a replacement directory on another disk.

If the same `.lumilioroot` appears at a new mount path or drive letter, add that directory again in Desktop and confirm **Reconnect here**. Child repository paths are updated while preserving their relative layout.

Removing an unused external Storage Location only removes its registration. It does not delete its directory, marker, or media. A location still referenced by registered repositories cannot be removed.

## Data that does not travel with a Storage Location

A Storage Location is not a complete workspace. These remain private to the machine running Lumilio:

- the PostgreSQL database;
- login keys and database credentials;
- cloud sessions and credential state;
- service logs, Lumen models, and database backups.

Repository-owned recoverable work remains under `.lumilio/`, including import staging and non-destructive edit state. Do not edit `.lumilioroot`, `.lumiliorepo`, or `.lumilio/` manually.

For a first test, use the primary repository in the default Storage Location. Once that works, test external storage with a small set of backed-up media using either **Add location → Create repository** or **Attach repository**.
