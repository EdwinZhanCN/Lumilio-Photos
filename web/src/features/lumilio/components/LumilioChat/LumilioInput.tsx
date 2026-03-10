import { RichInput, RichInputProvider } from "../RichInput";
import { MentionEntity, MentionType } from "../RichInput/types";
import { useMentionCatalog } from "../RichInput/useMentionCatalog";

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
  const { mentionTypes, entitiesByType, isLoading } = useMentionCatalog();

  const getEntitiesByType = (type: MentionType): MentionEntity[] =>
    entitiesByType[type] ?? [];

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
            : isGenerating
              ? "Generating response..."
              : isLoading
                ? "Loading mention catalog..."
                : null}
        </div>
      </div>
    </RichInputProvider>
  );
}
