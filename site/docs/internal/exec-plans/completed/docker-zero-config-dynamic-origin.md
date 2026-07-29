# Docker Zero-Configuration And Dynamic Origin

## Goal

Make the shared Server/Desktop runtime usable without a configured public URL
or mandatory proxy topology. The default Linux container starts from an
embedded complete schema-v3 manifest at `http://<host>:6680`; host networking
preserves Lumen mDNS discovery. Caddy and built-in ACME remain optional HTTPS
paths.

## Final Contracts

- Schema v3 changed in place without compatibility or migration code.
- `server.primary_origin`, `tls.mode = "external"`, and
  `server.proxy.mode` were removed.
- Browser origins and WebAuthn identity are derived per request. Challenges
  bind the normalized origin and RP ID used at registration or authentication.
- Forwarded host and scheme headers describe the request target but never
  authorize access. `trusted_cidrs` controls only forwarded client-IP recovery.
- The image embeds complete `docker-http` and `docker-caddy` manifests.
  Default, Caddy, and ACME Compose variants all use host networking.
- Desktop exposes local-only and LAN HTTP modes. HTTPS termination remains an
  optional deployment concern outside the control panel.

## Validation Boundary

Passed:

- `make config-examples`
- `make dto`
- `make server-test`
- `make web-test`
- `make desktop-test`
- `make compose-test`
- `vp run build` in `site/`
- strict validation of `web/e2e/support/server.e2e.toml`

`make web-browser-test` completed four of five smoke scenarios. The unrelated
Events scenario failed while rebuilding reused E2E state because
`event_media_items.media_item_id` hit its existing SQLite uniqueness
constraint. The origin, login, scan, and upload smoke scenarios passed.

## Useful Decisions

- Plain HTTP remains the zero-configuration baseline; users are not required
  to own a domain or provide an HTTPS URL.
- Passkeys are available only when the current browser origin satisfies the
  secure-origin and registrable-domain requirements.
- Product code, deployment files, generated contracts, and Web locales are
  submitted separately from the user-manual and VitePress changes.
