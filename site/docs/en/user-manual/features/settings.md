# Account, Users & Preferences

<DocPath :items="['Sidebar', 'Settings']" />

**Settings** organizes different kinds of configuration into tabs. Account and appearance are for every user; **AI** and **User management** are administrator-only. Check which tab you are on before saving, because some options affect only the current browser while others change the whole Server.

If no administrator can sign in to use account management, see [Recover administrator access](../introduction/break-glass).

## Account: protect your login first

The **Account** tab lets you:

- change your display name and avatar. The avatar is picked from existing media and never copies or deletes the original file;
- change your password. The current session signs out after a successful change and you must sign in again;
- enable or disable an authenticator app (TOTP), view remaining recovery codes, and regenerate them;
- register and remove passkeys. The current implementation requires TOTP to be enabled first, and the browser must support passkeys in a secure context.

Before replacing TOTP or passkeys, confirm that another login method and the recovery codes still work. Regenerating recovery codes invalidates the old ones immediately.

## Appearance: only changes this browser

The **Appearance** tab has three groups of preferences:

1. **Language and region**: interface language plus regional formats for dates and numbers;
2. **Theme pairing**: follow the operating system, or pick light and dark themes separately;
3. **Resource pages**: choose the **relaxed** or **compact** layout; the compact layout also lets you adjust the number of columns.

These preferences are stored in this browser's local settings. They need to be chosen again after switching browsers, clearing site data, or using another device; they never change media, repositories, or other users' settings.

## Cloud import: credentials and sessions only

In the **Cloud import** tab, click **Add credential**, pick a provider from the list, and enter the account information. Some providers ask for an SMS or other verification code; only after verification does the credential become **connected**.

- **Disconnect**: keeps the credential record but stops the current session; you can reconnect later;
- **Reconnect**: re-establishes a session with the saved credential;
- **Remove**: deletes the local credential and session data. Media already imported into a repository is not deleted.

Cloud credentials do not import anything by themselves. Create or open the destination repository separately, then click **Import from cloud** on its repository card.

::: warning Credentials and authorization
Enter cloud account passwords only on trusted Lumilio pages. Disconnecting or removing a local credential does not revoke other authorizations on the cloud provider's side; to fully revoke, also check the provider's account security page.
:::

## Server: runtime info and database backups

The **Server** tab contains:

- **Health check interval**: how often this browser checks whether the Server is online, 1–50 seconds;
- **Database backups**: enable automatic backups, choose an interval, set how many to keep, and create, download, restore, or delete a backup immediately;
- **Runtime configuration**: a read-only projection of the parsed TOML manifest, for example listen address, TLS, Default Storage Location, scanning, and Lumen Intelligence status.

In-app backups contain only the SQLite catalog data (albums, people, edit records, and so on) — not the original media files. Restore is **asynchronous**: clicking **Restore** only submits the request (the server returns `202 Accepted` with an operation ID); it does not mean the restore is complete. The page may briefly disconnect during the controlled restart, which is expected; you can use the operation ID to keep observing the same operation from the settings page. The flow moves through staged, restart, install, and verify; if verification fails, the previous database is restored. See [Backup and data integrity](../introduction/integrity). Before restoring, confirm the backup time and source are correct and that the original media and application data have their own independent backup.

The runtime configuration cannot be edited from the web page. Server deployers change the full TOML manifest and restart; Desktop paths, listening settings, certificates, and logs are managed by the Desktop Control Panel.

## AI: administrator-configured optional capabilities

Only administrators see the **AI** tab. It manages two independent capabilities: **Lumilio Agent** (the conversational organizing assistant) and **Lumen Intelligence** (media-understanding tasks). Both sections require **explicit choices**; there is no silent fallback to a default service.

### Choosing the LLM provider (Lumilio Agent)

- The provider must be **chosen explicitly**: none, unknown, or empty means the Agent is not configured and nothing falls back to a default provider;
- After picking the provider, enter the model name. **OpenAI-compatible providers such as DeepSeek and Ollama require an explicit Base URL** so traffic does not hit a wrong default endpoint; **remote providers require an API key**;
- **Verify connection** validates the current unsaved draft: nothing is saved unless verification passes, and a failed verification saves nothing;
- **Switching providers requires providing the secret again**: the saved API key belongs to the previous provider and is not carried over;
- “**Enable Lumilio Agent**” and “**Verify provider**” are two different actions: the enable switch only changes the Agent's availability, the verify button only checks the connection; the server also runs a structural integrity check again before enabling.

### Media-understanding tasks (Lumen Intelligence)

- The **ML** section enables Image Semantic Analysis, Video Semantics, BioCLIP Species Recognition, OCR Text Recognition, and Person Recognition tasks separately. Video semantics depends on Image Semantic Analysis; disabling the parent disables it too.

### After disabling a task

These switches never delete existing media or catalog data. After you disable a task, new tasks that depend on it stop queueing, but existing results may remain; after re-enabling, an administrator can rebuild indexes on demand from the [Server Monitor](./monitor) page.

Installing and reconfiguring Lumen Intelligence nodes themselves happens in the Desktop Control Panel, with `lumen-cli configure`, or in Docker — see [Lumen Intelligence](./lumen-intelligence).

## User management: changing access of existing accounts

In the **User management** tab, administrators can pick an existing account and change:

- username, display name, and avatar;
- the **User** or **Administrator** role;
- whether the account is active.

The interface lists each account's repository and album counts so you can confirm the target. There is no “delete user” action; deactivating an account blocks login but does not delete its media.

If another user lost all login factors, an administrator can use **Reset access** to generate a one-time temporary password. This clears that user's TOTP, recovery codes, and passkeys; pass the temporary password over a secure channel and ask the user to set a new password and verification method right after signing in. Administrators cannot reset their own account with this button.

## About

The **About** tab provides terms of use, open-source licenses, third-party notices, and project repository links. When reporting a problem, record the version and runtime info shown here first.

::: info The Desktop Control Panel is not in Web Settings
Desktop storage-location authorization, Server start/stop, Lumen Intelligence install and cache, certificates, logs, and runtime configuration that needs a restart all live in the **Desktop Control Panel**. The web **Settings** page only manages the account, preferences, and Server API configuration listed above; do not set the same item in both places.
:::
