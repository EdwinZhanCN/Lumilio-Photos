# Recover administrator access

Use BreakGlass only when an **active administrator** has lost every sign-in factor and no other administrator can use **Reset access**. It does not repair configuration, database, or startup failures.

BreakGlass replaces the administrator password, removes passkeys, TOTP and recovery codes, and invalidates existing sessions. The temporary password must be replaced immediately after sign-in.

::: danger Sensitive log
The temporary password is written only to `security.log`. Do not upload this file, paste it into an issue, or send it to a log collection service.
:::

## Docker Compose

Run these commands from the directory containing the Lumilio Photos Compose file.

1. Stop the normal server so two queue and API instances cannot run together:

   ```bash
   docker compose stop server
   ```

2. Start a one-time recovery container. Omit the username option to recover the oldest active administrator:

   ```bash
   docker compose run -d --name lumilio-breakglass \
     -e LUMILIO_BREAK_GLASS=true \
     -e LUMILIO_BREAK_GLASS_USERNAME=admin \
     server
   ```

3. Read the successful `auth.break_glass` event and copy its `temporary_password`:

   ```bash
   docker exec lumilio-breakglass cat /app/logs/security.log
   ```

4. Remove the one-time container and restart the normal server without BreakGlass:

   ```bash
   docker rm -f lumilio-breakglass
   docker compose up -d server
   ```

5. Sign in with the temporary password and choose a permanent password when prompted.

## Desktop

First quit Lumilio Photos completely from its menu-bar or tray icon. An existing instance will reject a recovery launch.

### macOS

```bash
open -n -a "Lumilio Photos" --args \
  --break-glass \
  --break-glass-username admin
```

The security log is:

```text
~/Library/Application Support/Lumilio Photos/logs/security.log
```

### Windows PowerShell

```powershell
& "$env:LOCALAPPDATA\Programs\Lumilio Photos\lumilio-photos.exe" `
  --break-glass `
  --break-glass-username admin
```

The security log is:

```text
%LOCALAPPDATA%\Lumilio Photos\logs\security.log
```

Omit `--break-glass-username admin` to recover the oldest active administrator. After copying the temporary password, quit this recovery launch and start Lumilio Photos normally. Then sign in and complete the required password change.

## If recovery fails

- The named account must exist, have the administrator role, and be active.
- On Desktop, verify that the existing tray application was fully closed.
- For Docker, check `docker logs lumilio-breakglass` for startup failures and wait for `security.log` to be created.
- If configuration loading, PostgreSQL, migrations, or security-log initialization fails, repair that startup problem first; BreakGlass runs only after those dependencies are ready.
