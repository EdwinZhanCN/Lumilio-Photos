# Authentication System — Business Flow Diagrams

## System Overview

```mermaid
graph TB
    subgraph Client["Frontend (React)"]
        LS[localStorage<br/>auth_token / refresh_token / media_token]
        WA[WebAuthn API<br/>navigator.credentials]
        AUTH_STATE[Auth State Machine<br/>AUTH_START → SUCCESS / FAILURE]
    end

    subgraph Server["Backend (Go + Gin)"]
        MW[Auth Middleware<br/>JWT Validation]
        AS[AuthService]
        US[UserService]
        subgraph Secrets["Key Derivation"]
            ROOT[LUMILIO_SECRET_KEY]
            JWT_K[jwt.signing.v1]
            MFA_K[mfa.signing.v1]
            PK_K[passkey.signing.v1]
            MEDIA_K[media.url.signing.v1]
            ENC_K[mfa.encryption.v1]
        end
    end

    subgraph DB["PostgreSQL"]
        USERS[users]
        RT[refresh_tokens]
        RS[registration_sessions]
        TOTP[user_mfa_totp_credentials]
        RC[user_mfa_recovery_codes]
        WC[user_webauthn_credentials]
    end

    Client -->|Bearer Token| MW
    MW --> AS
    AS --> DB
    US --> DB
    ROOT --> JWT_K & MFA_K & PK_K & MEDIA_K & ENC_K
```

---

## 1. User Registration (Staged Flow)

```mermaid
sequenceDiagram
    participant U as User (Browser)
    participant F as Frontend
    participant B as Backend
    participant DB as PostgreSQL

    Note over U,DB: Phase 1 — Create Registration Session

    U->>F: Enter username + password
    F->>B: POST /auth/register/start
    B->>B: normalizeUsername (3-32 chars, lowercase, letter-start)
    B->>B: validatePasswordPolicy (10-72 chars, upper+lower+digit)
    B->>DB: Check username uniqueness
    B->>DB: Clean expired sessions
    B->>B: bcrypt(password, cost=10)
    B->>B: Check bootstrap status (first user → admin)
    B->>B: Generate 32-byte webauthn_user_handle
    B->>DB: INSERT registration_sessions (TTL=20min)
    B-->>F: { registration_session_id, bootstrap_admin, next_role }

    Note over U,DB: Phase 2a — Complete via Passkey

    F->>B: POST /auth/passkeys/register/options<br/>{registration_session_id}
    B->>B: Create WebAuthn CreationOptions<br/>(ResidentKey=Required, UV=Required)
    B->>B: Issue challenge_token JWT (TTL=10min)
    B-->>F: { options, challenge_token }
    F->>U: Prompt biometric / security key
    U->>F: Authenticator response
    F->>B: POST /auth/passkeys/register/verify<br/>{session_id, challenge_token, credential}
    B->>B: Parse & validate attestation
    B->>DB: BEGIN TX
    B->>DB: INSERT users (role determined by bootstrap)
    B->>DB: INSERT user_webauthn_credentials
    B->>DB: DELETE registration_sessions
    B->>DB: COMMIT
    B->>B: Generate access_token + refresh_token
    B-->>F: { user, token, refreshToken, expiresAt }

    Note over U,DB: Phase 2b — Complete via TOTP

    F->>B: POST /auth/register/totp/setup<br/>{registration_session_id}
    B->>B: Generate TOTP secret
    B->>B: AES-GCM encrypt secret
    B->>DB: UPDATE registration_sessions SET totp_secret_ciphertext
    B-->>F: { secret, issuer, account_name, otpauth_uri }
    F->>U: Display QR code
    U->>F: Enter 6-digit TOTP code
    F->>B: POST /auth/register/totp/complete<br/>{registration_session_id, code}
    B->>B: Decrypt secret → validate TOTP code
    B->>B: Generate recovery codes (hash each)
    B->>DB: BEGIN TX
    B->>DB: INSERT users
    B->>DB: INSERT user_mfa_totp_credentials
    B->>DB: INSERT user_mfa_recovery_codes (×N)
    B->>DB: DELETE registration_sessions
    B->>DB: COMMIT
    B->>B: Generate access_token + refresh_token
    B-->>F: { auth: {user, token, refreshToken}, recovery_codes }
```

---

## 2. Login Flow (Password + MFA)

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant B as Backend
    participant DB as PostgreSQL

    U->>F: Enter username + password
    F->>B: POST /auth/login
    B->>DB: GetUserByUsername(lowercase)
    B->>B: Check is_active
    B->>B: bcrypt.Compare(password, hash)

    alt MFA NOT enabled
        B->>DB: UPDATE last_login
        B->>B: Generate access_token (15min) + refresh_token (7d)
        B->>DB: INSERT refresh_tokens
        B-->>F: { user, token, refreshToken, expiresAt }
        F->>F: Save to localStorage
    else MFA enabled (TOTP)
        B->>B: Issue MFA challenge JWT<br/>(purpose=mfa_login, TTL=5min)
        B-->>F: { requires_mfa: true, mfa_token, mfa_methods: ["totp","recovery_code"] }
        F->>U: Show MFA input
        U->>F: Enter TOTP code or recovery code
        F->>B: POST /auth/mfa/verify<br/>{mfa_token, code, method}

        alt method = "totp"
            B->>DB: Get TOTP credential (encrypted)
            B->>B: AES-GCM decrypt secret
            B->>B: Validate TOTP (±1 window)
        else method = "recovery_code"
            B->>B: SHA-256 hash input
            B->>DB: UseRecoveryCode (mark used_at)
        end

        B->>DB: UPDATE last_login
        B->>B: Generate tokens
        B-->>F: { user, token, refreshToken, expiresAt }
    end
```

---

## 3. Passkey Login Flow

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant B as Backend
    participant DB as PostgreSQL

    U->>F: Enter username, choose "Login with Passkey"
    F->>B: POST /auth/passkeys/login/options<br/>{username}
    B->>DB: GetUserByUsername
    B->>DB: ListUserWebAuthnCredentials
    B->>B: Build WebAuthn RequestOptions<br/>(UV=Required)
    B->>B: Issue challenge_token JWT (TTL=10min)
    B-->>F: { options (allowCredentials), challenge_token }
    F->>U: Prompt biometric / security key
    U-->>F: Authenticator assertion response
    F->>B: POST /auth/passkeys/login/verify<br/>{challenge_token, credential}
    B->>B: Parse challenge JWT → get userID
    B->>DB: Load user + passkeys
    B->>B: webauthn.ValidateLogin (verify signature)
    B->>DB: UPDATE sign_count, last_used_at
    B->>DB: UPDATE users SET last_login
    B->>B: Generate access_token + refresh_token
    B-->>F: { user, token, refreshToken, expiresAt }

    Note over F: Passkey login bypasses TOTP MFA
```

---

## 4. Token Lifecycle

```mermaid
stateDiagram-v2
    [*] --> Login: Credentials valid
    Login --> AccessToken: JWT issued (15min)
    Login --> RefreshToken: Random 32-byte hex (7d, stored in DB)

    AccessToken --> Expired: After 15 min
    Expired --> RefreshFlow: Client sends refreshToken

    RefreshFlow --> NewAccessToken: Validate DB record
    RefreshFlow --> NewRefreshToken: Rotation (old revoked)
    RefreshFlow --> Rejected: Token revoked/expired

    NewAccessToken --> Expired: After 15 min
    Rejected --> [*]: Force re-login

    state "Logout" as LO
    AccessToken --> LO: POST /auth/logout
    LO --> [*]: Revoke refreshToken in DB

    state "Password Change" as PC
    AccessToken --> PC: PATCH /users/me/password
    PC --> [*]: Revoke ALL user refresh tokens
```

```mermaid
graph LR
    subgraph "Token Types"
        AT["Access Token<br/>JWT HS256<br/>TTL: 15min"]
        RT["Refresh Token<br/>Random hex<br/>TTL: 7 days<br/>DB-stored, rotated"]
        MT["Media Token<br/>JWT HS256<br/>TTL: 10min<br/>scope: media"]
        MFA_T["MFA Token<br/>JWT HS256<br/>TTL: 5min<br/>purpose: mfa_login"]
        PK_T["Passkey Challenge<br/>JWT HS256<br/>TTL: 10min<br/>contains SessionData"]
    end
```

---

## 5. MFA Management (Authenticated User)

```mermaid
flowchart TD
    A[User logged in] --> B{Check MFA Status}
    B -->|GET /auth/mfa| C[MFAStatus:<br/>totp_enabled, passkey_count,<br/>recovery_codes_remaining]

    C --> D{Enable TOTP?}
    D -->|POST /auth/mfa/totp/setup| E[Generate secret<br/>Issue setup_token JWT<br/>Return otpauth URI]
    E --> F[User scans QR]
    F -->|POST /auth/mfa/totp/enable<br/>setup_token + code| G[Validate TOTP code]
    G --> H[AES-GCM encrypt secret → DB<br/>Generate recovery codes → DB]
    H --> I[Return recovery_codes + status]

    C --> J{Disable TOTP?}
    J -->|POST /auth/mfa/totp/disable<br/>current_password| K[bcrypt verify password]
    K --> L[DELETE totp_credentials<br/>DELETE recovery_codes]

    C --> M{Regenerate Recovery Codes?}
    M -->|POST /auth/mfa/recovery-codes/regenerate<br/>current_password| N[bcrypt verify]
    N --> O[DELETE old codes<br/>INSERT new hashed codes]
    O --> P[Return new plaintext codes]
```

---

## 6. Passkey Enrollment & Management

```mermaid
sequenceDiagram
    participant U as User
    participant F as Frontend
    participant B as Backend

    Note over U,B: Enroll new passkey (already logged in)

    F->>B: POST /auth/mfa/passkeys/options
    B->>B: Load existing credentials (for excludeCredentials)
    B->>B: Create WebAuthn CreationOptions
    B->>B: Issue challenge JWT (TTL=10min)
    B-->>F: { options, challenge_token }
    F->>U: Prompt biometric
    U-->>F: Authenticator attestation
    F->>B: POST /auth/mfa/passkeys/verify<br/>{challenge_token, credential}
    B->>B: Validate challenge JWT (userID match)
    B->>B: webauthn.CreateCredential
    B->>B: Store credential in DB
    B-->>F: { passkey_id, label, transports, created_at }

    Note over U,B: List passkeys

    F->>B: GET /auth/mfa/passkeys
    B-->>F: { credentials: [...], total }

    Note over U,B: Delete passkey

    F->>B: DELETE /auth/mfa/passkeys/:id
    B->>B: Verify ownership (user_id match)
    B-->>F: { deleted: true }
```

---

## 7. Admin User Management

```mermaid
flowchart TD
    subgraph "Admin Operations (RequireAdmin middleware)"
        A[GET /users] --> B[List all users<br/>with asset_count, album_count]

        C[PATCH /users/:id] --> D{Changing role or is_active?}
        D -->|Demoting/disabling admin| E{Is last active admin?}
        E -->|Yes| F[❌ ErrCannotDisableLastAdmin]
        E -->|No| G[✅ Update user]
        D -->|Other changes| G

        H[POST /users/:id/reset-access] --> I[Generate temp password<br/>Lm9 + 21 random chars]
        I --> J[bcrypt hash temp password]
        J --> K[BEGIN TX]
        K --> L[UPDATE password]
        L --> M[DELETE all passkeys]
        M --> N[DELETE TOTP credential]
        N --> O[DELETE recovery codes]
        O --> P[REVOKE all refresh tokens]
        P --> Q[COMMIT]
        Q --> R[Return temp password to admin]
    end
```

---

## 8. Role & Permission Model

```mermaid
graph TD
    subgraph Roles
        ADMIN[admin]
        USER[user]
    end

    subgraph Permissions
        P1[manage_users]
        P2[manage_settings]
        P3[view_all_assets]
        P4[manage_all_assets]
        P5[view_own_assets]
        P6[manage_own_assets]
        P7[manage_own_profile]
    end

    ADMIN --> P1 & P2 & P3 & P4 & P7
    USER --> P5 & P6 & P7

    subgraph "Middleware Chain"
        REQ[Request] --> AUTH_MW[AuthMiddleware<br/>Validate JWT]
        AUTH_MW --> SET_CTX[Set user context:<br/>user_id, role, permissions]
        SET_CTX --> ADMIN_MW{RequireAdmin?}
        ADMIN_MW -->|Yes| CHECK[Check role == admin]
        ADMIN_MW -->|No| HANDLER[Handler]
        CHECK -->|Pass| HANDLER
        CHECK -->|Fail| DENY[403 Forbidden]
    end
```

---

## 9. Bootstrap Flow (First User)

```mermaid
flowchart TD
    START[Application Start] --> CHECK{GET /auth/bootstrap-status}
    CHECK -->|has_users: false| BOOTSTRAP[Bootstrap Mode<br/>next_role: admin]
    CHECK -->|has_users: true| NORMAL[Normal Mode<br/>next_role: user]

    BOOTSTRAP --> REG[Register first user]
    REG --> ADMIN_CREATED[First user becomes admin<br/>No invitation required]

    NORMAL --> REG2[Register subsequent user]
    REG2 --> USER_CREATED[New user gets 'user' role]
```

---

## 10. Secret Key Architecture

```mermaid
graph TD
    ENV["LUMILIO_SECRET_KEY<br/>(env var or auto-generated)"]
    ENV -->|SHA256 scope derive| S1["jwt.signing.v1<br/>Access Token signing"]
    ENV -->|SHA256 scope derive| S2["mfa.signing.v1<br/>MFA challenge JWT signing"]
    ENV -->|SHA256 scope derive| S3["passkey.signing.v1<br/>Passkey challenge JWT signing"]
    ENV -->|SHA256 scope derive| S4["media.url.signing.v1<br/>Media token signing"]
    ENV -->|SHA256 scope derive| S5["mfa.encryption.v1<br/>AES-256-GCM key for TOTP secrets"]

    S1 --> AT[Access Token JWT]
    S2 --> MFA[MFA Login Token / TOTP Setup Token]
    S3 --> PK[Passkey Challenge Token]
    S4 --> MT[Media Token JWT]
    S5 --> ENC[Encrypt/Decrypt TOTP secret_ciphertext]

    style ENV fill:#f96,stroke:#333
    style S5 fill:#69f,stroke:#333
```

---

## API Route Map

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| GET | `/auth/bootstrap-status` | — | Check if first-user bootstrap mode |
| POST | `/auth/register/start` | — | Create registration session |
| POST | `/auth/register/totp/setup` | — | Begin TOTP during registration |
| POST | `/auth/register/totp/complete` | — | Complete registration with TOTP |
| POST | `/auth/passkeys/register/options` | — | WebAuthn creation options (registration) |
| POST | `/auth/passkeys/register/verify` | — | Verify passkey attestation (registration) |
| POST | `/auth/login` | — | Password login |
| POST | `/auth/passkeys/login/options` | — | WebAuthn request options (login) |
| POST | `/auth/passkeys/login/verify` | — | Verify passkey assertion (login) |
| POST | `/auth/mfa/verify` | — | Verify MFA challenge |
| POST | `/auth/refresh` | — | Refresh access token |
| POST | `/auth/logout` | — | Revoke refresh token |
| GET | `/auth/me` | Bearer | Get current user |
| GET | `/auth/media-token` | Bearer | Get short-lived media token |
| GET | `/auth/mfa` | Bearer | Get MFA status |
| POST | `/auth/mfa/totp/setup` | Bearer | Begin TOTP setup |
| POST | `/auth/mfa/totp/enable` | Bearer | Enable TOTP |
| POST | `/auth/mfa/totp/disable` | Bearer | Disable TOTP (requires password) |
| POST | `/auth/mfa/recovery-codes/regenerate` | Bearer | Regenerate recovery codes |
| GET | `/auth/mfa/passkeys` | Bearer | List enrolled passkeys |
| POST | `/auth/mfa/passkeys/options` | Bearer | Begin passkey enrollment |
| POST | `/auth/mfa/passkeys/verify` | Bearer | Complete passkey enrollment |
| DELETE | `/auth/mfa/passkeys/:id` | Bearer | Delete a passkey |
| PATCH | `/users/me/profile` | Bearer | Update own profile |
| PATCH | `/users/me/password` | Bearer | Change own password |
| GET | `/users` | Admin | List all users |
| PATCH | `/users/:id` | Admin | Update any user |
| POST | `/users/:id/reset-access` | Admin | Reset user credentials |
