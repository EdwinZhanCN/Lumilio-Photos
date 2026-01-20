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
}

export function LumilioInput({
  isGenerating,
  isInitializing,
  onSubmit,
  commands = [],
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
          placeholder="Ask Lumilio Agent... (Type @ or /)"
          onSubmit={onSubmit}
          mentionTypes={mentionTypes}
          getEntitiesByType={getEntitiesByType}
          commands={commands}
          isSubmitting={isGenerating || isInitializing}
        />

        {/* Status info */}
        <div className="text-xs text-base-content/60 mt-2">
          {isGenerating && "Generating response..."}
        </div>
      </div>
    </RichInputProvider>
  );
}
