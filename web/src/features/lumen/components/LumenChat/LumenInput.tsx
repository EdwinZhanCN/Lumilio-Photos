interface LumenInputProps {
  inputMessage: string;
  isGenerating: boolean;
  isInitializing: boolean;
  onInputChange: (value: string) => void;
  onSendMessage: () => void;
  onKeyPress: (e: React.KeyboardEvent) => void;
}

export function LumenInput({
  inputMessage,
  isGenerating,
  isInitializing,
  onInputChange,
  onSendMessage,
  onKeyPress,
}: LumenInputProps) {
  return (
    <div className="p-4 border-t border-base-300">
      <div className="flex gap-2">
        <textarea
          className="textarea textarea-bordered flex-1 resize-none"
          placeholder="Type your message... (Press Enter to send, Shift+Enter for new line)"
          value={inputMessage}
          onChange={(e) => onInputChange(e.target.value)}
          onKeyPress={onKeyPress}
          disabled={isGenerating}
          rows={1}
        />
        <button
          className="btn btn-primary"
          onClick={onSendMessage}
          disabled={!inputMessage.trim() || isGenerating || isInitializing}
        >
          {isGenerating ? (
            <span className="loading loading-spinner loading-sm"></span>
          ) : (
            "Send"
          )}
        </button>
      </div>

      {/* Status info */}
      <div className="text-xs text-base-content/60 mt-2">
        {isInitializing && "Initializing model..."}
        {isGenerating && "Generating response..."}
      </div>
    </div>
  );
}
