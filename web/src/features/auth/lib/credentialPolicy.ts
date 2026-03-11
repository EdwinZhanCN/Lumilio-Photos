export const USERNAME_PATTERN =
  "[a-z](?:[a-z0-9]|[._-](?=[a-z0-9])){2,31}";
export const USERNAME_MIN_LENGTH = 3;
export const USERNAME_MAX_LENGTH = 32;
export const DISPLAY_NAME_MAX_LENGTH = 64;
export const PASSWORD_PATTERN = "(?=.*[a-z])(?=.*[A-Z])(?=.*\\d).{10,72}";
export const PASSWORD_MIN_LENGTH = 10;
export const PASSWORD_MAX_LENGTH = 72;

export const USERNAME_HINT =
  "3-32 characters, starting with a letter. Lowercase letters, numbers, dots, underscores, and hyphens are allowed.";
export const DISPLAY_NAME_HINT =
  "Display names support multiple languages and stay under 64 characters.";
export const PASSWORD_HINT =
  "10-72 characters with at least one uppercase letter, one lowercase letter, and one number.";

export function normalizeUsernameInput(value: string) {
  return value.toLowerCase();
}
