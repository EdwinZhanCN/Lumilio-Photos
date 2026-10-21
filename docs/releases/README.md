# Release notes

One file per published tag, written for users (grouped New / Fixed / Known
issues / Upgrading, product terms from the terminology registry in
`.agents/skills/lumilio-frontend-i18n/SKILL.md`, no internal names):

- `<tag>.md` — English
- `<tag>.zh-cn.md` — Simplified Chinese

The release workflow (`.github/workflows/release.yml`, "Write release
introduction") appends both files, when they exist for the pushed tag, to the
GitHub Release body after the generated introduction. The documentation site
renders the same files at `user-manual/introduction/release-notes` (en) and
`zh-cn/user-manual/reference/release-notes` (zh-cn) through VitePress
includes, so the notes are written once. Links inside the notes are absolute
`https://docs.lumilio.org/...` URLs because the files render both on GitHub
and on the site.

A session that changes user-visible behaviour before its release is tagged
adds or edits the matching line in the same PR.
