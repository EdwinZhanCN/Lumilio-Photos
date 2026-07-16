export type PasswordStrength = { score: number; label: string };

const STRENGTH_LABELS = ["Too short", "Weak", "Fair", "Good", "Strong"];

export function passwordStrength(pw: string): PasswordStrength {
  if (!pw) return { score: 0, label: "" };
  let s = 0;
  if (pw.length >= 8) s++;
  if (pw.length >= 12) s++;
  if (/[a-z]/.test(pw) && /[A-Z]/.test(pw)) s++;
  if (/\d/.test(pw)) s++;
  if (/[^A-Za-z0-9]/.test(pw)) s++;
  s = Math.min(s, 4);
  return { score: s, label: STRENGTH_LABELS[s] };
}
