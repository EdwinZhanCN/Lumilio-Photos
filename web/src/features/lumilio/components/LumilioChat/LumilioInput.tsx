import { RichInput, RichInputProvider } from "../RichInput";
import {
  MentionEntity,
  MentionType,
  MentionTypeOption,
} from "../RichInput/types";

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
  // Mention feature remains a placeholder for now
  const mentionTypes: MentionTypeOption[] = [
    { type: "placeholder", label: "Placeholder" },
  ];

  const getEntitiesByType = (type: MentionType) => {
    const entities: Record<MentionType, MentionEntity[]> = {
      placeholder: [
        {
          id: "placeholder-123",
          label: "Placeholder Label",
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
              ? "Lumilio Agent is unavailable."
              : "Ask Lumilio Agent... (Type @ or /)"
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
            ? (disabledHint ?? "Lumilio Agent is unavailable.")
            : isGenerating && "Generating response..."}
        </div>
      </div>
    </RichInputProvider>
  );
}
