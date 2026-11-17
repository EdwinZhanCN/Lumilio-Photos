/**
 * Canonical bilingual product terms from the terminology registry in
 * `.agents/skills/lumilio-frontend-i18n/SKILL.md`. They are frozen product
 * vocabulary guarded by the terminology gates, not UI copy, so a spec may
 * assert that a translated page uses them.
 */
export const TERMS = {
  storageLocation: { en: "Storage Location", zh: "存储位置" },
  repository: { en: "Repository", zh: "资源库" },
} as const;
