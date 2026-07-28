import { XIcon } from "lucide-react";
import { memo } from "react";
import { useI18n } from "@/lib/i18n";
import type { MediaComposition, StackKind, StackMembership } from "../../../../model/filter";
import type { MediaTypeFilter } from "../types";
import { SectionShell } from "./SectionShell";

interface ChoiceOption<Value extends string> {
  key: Value;
  label: string;
}

/**
 * Which control a section gets is decided by **label width**, not option count:
 * a row of pills only stays readable while every label is short. Rating has the
 * most options of any section yet is the best fit for pills, because each label
 * is one character; Composition and Grouping have fewer options but four long
 * ones, which would wrap to three ragged lines inside the 320px panel.
 *
 * The reset is a real `<button>`, not daisyUI's `.filter-reset` radio, because
 * that radio draws its glyph through `::after` and an `<input>` can never hold
 * an icon component. Dropping it is safe: daisyUI's collapse rule keys off
 * `input:checked:not(.filter-reset)` and behaves identically without one, and
 * "no selection" is simply every radio unchecked.
 *
 * It collapses by animating a wrapping grid track to `0fr` instead of shrinking
 * the button. Sizing the button itself is what went wrong twice before —
 * daisyUI hides `.filter-reset` with `width: 0`, which any width utility
 * (`btn-square`, `shrink-0`) silently overrides, leaving an invisible block that
 * indents the row — while unmounting it removed the gap but lost the animation.
 * A grid track owns the width outright, so the button keeps its natural size and
 * still animates in and out.
 */
function ChoicePills<Value extends string>({
  name,
  options,
  value,
  onValueChange,
  locked,
  clearLabel,
}: {
  name: string;
  options: ChoiceOption<Value>[];
  value: Value | undefined;
  onValueChange: (value: Value | undefined) => void;
  locked: boolean;
  clearLabel: string;
}) {
  const showReset = value !== undefined && !locked;

  return (
    <div className="filter">
      {/*
        Collapsing a grid track rather than the button itself keeps the reset
        mounted so it can animate in and out, without any width utility that
        could fight the button's own sizing and leave an invisible block behind.
      */}
      <span
        className={`grid transition-[grid-template-columns,opacity,margin] duration-200 ease-out ${
          showReset
            ? "me-1 grid-cols-[1fr] opacity-100"
            : "pointer-events-none grid-cols-[0fr] opacity-0"
        }`}
        aria-hidden={!showReset}
      >
        <span className="overflow-hidden">
          <button
            type="button"
            className="btn btn-xs btn-circle btn-ghost"
            aria-label={clearLabel}
            tabIndex={showReset ? 0 : -1}
            onClick={() => onValueChange(undefined)}
          >
            <XIcon className="size-3" />
          </button>
        </span>
      </span>
      {options.map((option) => (
        <input
          key={option.key}
          className="btn btn-xs"
          type="radio"
          name={name}
          aria-label={option.label}
          checked={value === option.key}
          disabled={locked}
          onChange={() => onValueChange(option.key)}
        />
      ))}
    </div>
  );
}

/** One line, never wraps, never clips — the right shape for long labels. */
function ChoiceSelect<Value extends string>({
  options,
  value,
  onValueChange,
  locked,
  allLabel,
  ariaLabel,
}: {
  options: ChoiceOption<Value>[];
  value: Value | undefined;
  onValueChange: (value: Value | undefined) => void;
  locked: boolean;
  allLabel: string;
  ariaLabel: string;
}) {
  return (
    <select
      className="select select-bordered select-xs w-full"
      aria-label={ariaLabel}
      disabled={locked}
      value={value ?? ""}
      onChange={(event) => onValueChange((event.target.value || undefined) as Value | undefined)}
    >
      <option value="">{allLabel}</option>
      {options.map((option) => (
        <option key={option.key} value={option.key}>
          {option.label}
        </option>
      ))}
    </select>
  );
}

interface TypeSectionProps {
  locked: boolean;
  value: MediaTypeFilter | undefined;
  onValueChange: (value: MediaTypeFilter | undefined) => void;
}

export const TypeSection = memo(function TypeSection({
  locked,
  value,
  onValueChange,
}: TypeSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell title={t("assets.filterTool.typeSection.title")} locked={locked}>
      <ChoicePills
        name="filter-type"
        locked={locked}
        value={value}
        onValueChange={onValueChange}
        clearLabel={t("assets.filterTool.common.clear_choice", "Clear this filter")}
        options={[
          { key: "PHOTO", label: t("assets.filterTool.typeSection.photo") },
          { key: "VIDEO", label: t("assets.filterTool.typeSection.video") },
        ]}
      />
    </SectionShell>
  );
});

interface CompositionSectionProps {
  locked: boolean;
  value: MediaComposition | undefined;
  onValueChange: (value: MediaComposition | undefined) => void;
}

export const CompositionSection = memo(function CompositionSection({
  locked,
  value,
  onValueChange,
}: CompositionSectionProps) {
  const { t } = useI18n();
  const title = t("assets.filterTool.compositionSection.title", "Media composition");

  return (
    <SectionShell title={title} locked={locked}>
      <ChoiceSelect
        locked={locked}
        value={value}
        onValueChange={onValueChange}
        ariaLabel={title}
        allLabel={t("assets.filterTool.common.all", "All")}
        options={[
          {
            key: "contains_raw",
            label: t("assets.filterTool.compositionSection.contains_raw", "Contains RAW"),
          },
          {
            key: "jpeg_raw",
            label: t("assets.filterTool.compositionSection.jpeg_raw", "JPEG + RAW"),
          },
          {
            key: "raw_unpaired",
            label: t("assets.filterTool.compositionSection.raw_unpaired", "Unpaired RAW"),
          },
          { key: "no_raw", label: t("assets.filterTool.compositionSection.no_raw", "No RAW") },
          {
            key: "live_photo",
            label: t("assets.filterTool.compositionSection.live_photo", "Live Photo"),
          },
        ]}
      />
    </SectionShell>
  );
});

type StackChoice = "unstacked" | "stacked" | "burst" | "manual";

export interface StackSelection {
  membership?: StackMembership;
  kinds: StackKind[];
}

const STACK_SELECTIONS: Record<StackChoice, StackSelection> = {
  unstacked: { membership: "unstacked", kinds: [] },
  stacked: { membership: "stacked", kinds: [] },
  burst: { kinds: ["burst"] },
  manual: { kinds: ["manual"] },
};

const CLEARED_STACK_SELECTION: StackSelection = { kinds: [] };

/** Picking a single kind already implies "stacked", so both kinds selected reads as "all stacks". */
function toStackChoice(selection: StackSelection): StackChoice | undefined {
  if (selection.membership === "unstacked") return "unstacked";
  if (selection.kinds.length === 1) return selection.kinds[0];
  if (selection.membership === "stacked" || selection.kinds.length > 1) return "stacked";
  return undefined;
}

interface StackSectionProps {
  locked: boolean;
  value: StackSelection;
  onValueChange: (value: StackSelection) => void;
}

export const StackSection = memo(function StackSection({
  locked,
  value,
  onValueChange,
}: StackSectionProps) {
  const { t } = useI18n();
  const title = t("assets.filterTool.stackSection.title", "Grouping");

  return (
    <SectionShell title={title} locked={locked}>
      <ChoiceSelect
        locked={locked}
        value={toStackChoice(value)}
        onValueChange={(choice) =>
          onValueChange(choice ? STACK_SELECTIONS[choice] : CLEARED_STACK_SELECTION)
        }
        ariaLabel={title}
        allLabel={t("assets.filterTool.common.all", "All")}
        options={[
          { key: "unstacked", label: t("assets.filterTool.stackSection.unstacked", "Ungrouped") },
          { key: "stacked", label: t("assets.filterTool.stackSection.stacked", "All stacks") },
          { key: "burst", label: t("assets.filterTool.stackSection.burst", "Burst") },
          { key: "manual", label: t("assets.filterTool.stackSection.manual", "Manual stack") },
        ]}
      />
    </SectionShell>
  );
});

interface RatingSectionProps {
  locked: boolean;
  value: number | undefined;
  onValueChange: (value: number | undefined) => void;
}

export const RatingSection = memo(function RatingSection({
  locked,
  value,
  onValueChange,
}: RatingSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell title={t("assets.filterTool.ratingSection.title")} locked={locked}>
      <ChoicePills
        name="filter-rating"
        locked={locked}
        value={value === undefined ? undefined : String(value)}
        onValueChange={(next) => onValueChange(next === undefined ? undefined : Number(next))}
        clearLabel={t("assets.filterTool.common.clear_choice", "Clear this filter")}
        options={[
          ...[5, 4, 3, 2, 1].map((rating) => ({ key: String(rating), label: String(rating) })),
          { key: "0", label: t("assets.filterTool.ratingSection.unrated_short") },
        ]}
      />
    </SectionShell>
  );
});

interface LikeSectionProps {
  locked: boolean;
  value: boolean | undefined;
  onValueChange: (value: boolean | undefined) => void;
}

export const LikeSection = memo(function LikeSection({
  locked,
  value,
  onValueChange,
}: LikeSectionProps) {
  const { t } = useI18n();

  return (
    <SectionShell title={t("assets.filterTool.likeSection.title")} locked={locked}>
      <ChoicePills
        name="filter-liked"
        locked={locked}
        value={value === undefined ? undefined : value ? "liked" : "unliked"}
        onValueChange={(next) => onValueChange(next === undefined ? undefined : next === "liked")}
        clearLabel={t("assets.filterTool.common.clear_choice", "Clear this filter")}
        options={[
          { key: "liked", label: t("assets.filterTool.likeSection.liked") },
          { key: "unliked", label: t("assets.filterTool.likeSection.unliked") },
        ]}
      />
    </SectionShell>
  );
});
