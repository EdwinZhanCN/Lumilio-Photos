// Package locales holds every string a person reads in the Desktop App: the
// Settings window, the tray, and the window title.
//
// The catalogs are one ICU MessageFormat file per language, named after its
// BCP 47 tag; en.json is the source and describes typed placeholders.
// catalog_gen.go is written from them by `mygo generate` and gives each
// message a typed function, which says it in the language the app shows.
// It is generated code: change a catalog, regenerate, and commit both.
//
// Product terms follow the canonical bilingual terminology registry in
// .agents/skills/lumilio-frontend-i18n/SKILL.md.
package locales
