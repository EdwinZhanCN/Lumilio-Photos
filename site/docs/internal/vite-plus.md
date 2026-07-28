<!--VITE PLUS START-->

# Using Vite+, the Unified Toolchain for the Web

This project's web frontend is using Vite+, a unified toolchain built on top of Vite, Rolldown, Vitest, tsdown, Oxlint, Oxfmt, and Vite Task. Vite+ wraps runtime management, package management, and frontend tooling in a single global CLI called `vp`. Vite+ is distinct from Vite, and it invokes Vite through `vp dev` and `vp build`. Run `vp help` to print a list of commands and `vp <command> --help` for information about a specific command.

Docs are local at `web/node_modules/vite-plus/docs` or online at https://viteplus.dev/guide/.

## Review Checklist

- [ ] Run `vp install` after pulling remote changes and before getting started.
- [ ] Prefer the repo gate `make web-test`. If intentionally scoped to `web/`,
      run the same sequence directly: `vp check --no-fmt --no-lint`, `vp lint`,
      then `vp test`.
- [ ] Treat `vp fmt` as a write command. Use `vp fmt --check` for a dry run.
      Keep generated/vendored artifacts out of formatting through
      `web/vite.config.ts` `fmt.ignorePatterns`: generated `doc.md`, WASM
      bundles, OpenAPI schema, and vendored OpenAPI client helpers should only
      change through their generator or source update flow.
- [ ] Check if there are `vite.config.ts` tasks or `package.json` scripts necessary for validation, run via `vp run <script>`.
- [ ] If setup, runtime, or package-manager behavior looks wrong, run `vp env doctor` and include its output when asking for help.

<!--VITE PLUS END-->
