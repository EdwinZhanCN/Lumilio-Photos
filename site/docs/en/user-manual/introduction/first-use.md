# First-time setup

After installing, first-time setup does three things: **create the admin → set up login protection → create the primary repository**. Desktop enters through the Control Panel; Docker opens the Server address directly.

## Before you start

- Desktop: the Control Panel shows the Server as running and the browser has opened `http://localhost:6680`.
- Docker: `docker compose ps` shows `lumilio` healthy; open `http://<host-IP>:6680` in the browser.

The “First run setup” welcome page means you can start.

## Step 1: Create the admin

1. Choose the interface language and region (browser preference only).
2. Enter a username (3–32 chars, starting with a letter) and a password (10–72 chars, with upper case, lower case, and a digit).
3. Click **Create admin and continue**.

**Success**: you are signed in and moved to the security step.

## Step 2: Set up login protection (recommended)

The wizard offers TOTP, a passkey, and recovery codes.

- **Enable TOTP**: scan the QR code with an authenticator app (Apple Passwords, Google Authenticator, Microsoft Authenticator) and enter the 6-digit code.
- **Passkey**: optional; available after TOTP, only on localhost or an HTTPS address.
- **Recovery codes**: one-time codes shown after TOTP; **store them somewhere safe, separate from the device**.

You can skip, but the admin controls the whole Server — skipping means password-only protection.

## Step 3: Create the primary repository

1. Enter a **name** (display only; the directory is always `primary`).
2. **Storage layout**: keep the default `date` (archive by year/month).
3. **Filename conflict handling**: keep the default `rename` (safe rename on conflict; originals are never replaced).

**Success**: the main interface opens.

## Verify

1. Open **Manage** and confirm the primary repository is available;
2. Add a few photos and wait for processing;
3. Open the library and confirm viewing works;
4. Create your first database backup (see [Backup and Data Integrity](./integrity)).

## If it fails

- Desktop: check the Control Panel Server status and error logs.
- Docker: `docker compose logs lumilio`.
- Never create or delete the `primary` directory manually; determine whether the problem is the missing admin, the missing primary repository, or an unwritable storage path.
