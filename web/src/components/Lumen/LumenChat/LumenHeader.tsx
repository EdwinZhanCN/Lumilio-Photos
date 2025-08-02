interface LumenHeaderProps {
  currentModelId: string | null;
  selectedModel: string;
  availableModels: Array<{ id: string; name: string }>;
  isInitializing: boolean;
  isGenerating: boolean;
  conversationLength: number;
  isNoThink: boolean;
  onModelChange: (modelId: string) => void;
  onClearConversation: () => void;
  onCancelGeneration: () => void;
  onToggleThink: (enabled: boolean) => void;
}

export function LumenHeader({
  currentModelId,
  selectedModel,
  availableModels,
  isInitializing,
  isGenerating,
  conversationLength,
  isNoThink,
  onModelChange,
  onClearConversation,
  onCancelGeneration,
  onToggleThink,
}: LumenHeaderProps) {
  return (
    <div className="flex items-center justify-between p-4 border-b border-base-300">
      <div>
        <h2 className="text-xl font-semibold">Lumen</h2>
        {currentModelId && (
          <p className="text-sm text-base-content/60">
            Using:{" "}
            {availableModels.find((m) => m.id === currentModelId)?.name ||
              currentModelId}
          </p>
        )}
      </div>
      <div className="flex gap-2 items-center">
        <div className="flex items-center gap-2">
          <label className="label cursor-pointer gap-2">
            <span className="label-text text-sm">Think</span>
            <input
              type="checkbox"
              className="toggle toggle-success"
              checked={!isNoThink}
              onChange={(e) => onToggleThink(!e.target.checked)}
              disabled={isInitializing || isGenerating}
            />
          </label>
        </div>
        <select
          className="select select-sm select-bordered"
          value={selectedModel}
          onChange={(e) => onModelChange(e.target.value)}
          disabled={isInitializing || isGenerating}
        >
          {availableModels.map((model) => (
            <option key={model.id} value={model.id}>
              {model.name}
            </option>
          ))}
        </select>
        <button
          className="btn btn-sm btn-outline"
          onClick={onClearConversation}
          disabled={isGenerating || conversationLength === 0}
        >
          Clear Chat
        </button>
        {isGenerating && (
          <button className="btn btn-sm btn-error" onClick={onCancelGeneration}>
            Cancel
          </button>
        )}
      </div>
    </div>
  );
}
