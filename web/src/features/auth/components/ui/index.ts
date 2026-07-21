/**
 * Shared authentication UI primitives.
 *
 * These mirror the Lumilio auth design handoff (daisyUI semantic roles mapped
 * onto the app's own `lumilio` theme). They are presentational only — every
 * API call stays in the feature hooks/pages so the same kit serves login,
 * register, bootstrap, and security-settings flows.
 */

export { cx } from "./classNames";
export { passwordStrength } from "./passwordStrength";
export { Brand, AuthShell, CardHead } from "./Shell";
export type { HeadTone } from "./Shell";
export { Field, TextInput, PasswordField } from "./Fields";
export { Btn, CopyButton } from "./Buttons";
export type { BtnVariant } from "./Buttons";
export { OtpInput, AuthQR } from "./Verification";
export { FlowSteps, Stepper } from "./Progress";
export type { StepperStep } from "./Progress";
export { PasskeyAffordance, RecoveryCodesPanel, TotpSetupPanel } from "./SecurityPanels";
export { InlineError, SuccessCard } from "./Feedback";
