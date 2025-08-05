interface LumenHeaderProps {
  currentModelId: string | null;
  isInitializing: boolean;
  isGenerating: boolean;
  conversationLength: number;
  isNoThink: boolean;
  onClearConversation: () => void;
  onCancelGeneration: () => void;
  onToggleThink: (enabled: boolean) => void;
}

export function LumenHeader({
  currentModelId,
  isInitializing,
  isGenerating,
  conversationLength,
  isNoThink,
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
            Using: {currentModelId}
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
