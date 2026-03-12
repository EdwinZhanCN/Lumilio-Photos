import { RichInput, RichInputProvider } from "../RichInput";
import {
  MentionEntity,
  MentionType,
  MentionTypeOption,
} from "../RichInput/types";
import { useI18n } from "@/lib/i18n.tsx";

interface LumilioInputProps {
  isGenerating: boolean;
  isInitializing: boolean;
  onSubmit: (value: string) => void;
  commands?: MentionEntity[];
  disabled?: boolean;
  disabledHint?: string;
}

export function LumilioInput({
  isGenerating,
  isInitializing,
  onSubmit,
  commands = [],
  disabled = false,
  disabledHint,
}: LumilioInputProps) {
  const { t } = useI18n();
  // Mention feature remains a placeholder for now
  const mentionTypes: MentionTypeOption[] = [
    { type: "placeholder", label: t("lumilio.input.placeholderType") },
  ];

  const getEntitiesByType = (type: MentionType) => {
    const entities: Record<MentionType, MentionEntity[]> = {
      placeholder: [
        {
          id: "placeholder-123",
          label: t("lumilio.input.placeholderLabel"),
          type: "placeholder",
        },
      ],
    };
    return entities[type] || [];
  };

  return (
    <RichInputProvider>
      <div className="p-4 border-t border-base-300">
        <RichInput
          placeholder={
            disabled
              ? t("lumilio.input.unavailable")
              : t("lumilio.input.prompt")
          }
          onSubmit={onSubmit}
          mentionTypes={mentionTypes}
          getEntitiesByType={getEntitiesByType}
          commands={commands}
          isSubmitting={isGenerating || isInitializing}
          isDisabled={disabled}
        />

        {/* Status info */}
        <div className="text-xs text-base-content/60 mt-2">
          {disabled
            ? (disabledHint ?? t("lumilio.input.unavailable"))
            : isGenerating && t("lumilio.input.generating")}
        </div>
      </div>
    </RichInputProvider>
  );
}
