import { FormEvent, KeyboardEvent, useState } from "react";
import { Send } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";

interface ChatInputProps {
  isGenerating: boolean;
  disabled?: boolean;
  disabledHint?: string;
  onSubmit: (value: string) => void;
}

export function ChatInput({
  isGenerating,
  disabled = false,
  disabledHint,
  onSubmit,
}: ChatInputProps) {
  const { t } = useI18n();
  const [value, setValue] = useState("");
  const isDisabled = isGenerating || disabled;

  const submit = (event?: FormEvent) => {
    event?.preventDefault();
    const trimmed = value.trim();
    if (!trimmed || isDisabled) return;
    onSubmit(trimmed);
    setValue("");
  };

  const handleKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key !== "Enter" || event.shiftKey) return;
    submit();
  };

  return (
    <form
      className="flex items-center gap-2.5 rounded-full border border-base-300 bg-base-100 pl-4 pr-2 py-1.5"
      onSubmit={submit}
    >
      <input
        className="input input-ghost h-9 min-w-0 flex-1 px-0 text-sm focus:outline-none"
        value={value}
        disabled={isDisabled}
        title={disabled ? disabledHint : undefined}
        placeholder={
          disabled ? t("lumilio.input.unavailable") : t("lumilio.input.prompt")
        }
        onChange={(event) => setValue(event.target.value)}
        onKeyDown={handleKeyDown}
      />
      <button
        type="submit"
        className="btn btn-primary btn-circle btn-md shrink-0"
        disabled={!value.trim() || isDisabled}
        title={t("lumilio.input.send", "Send")}
      >
        <Send size={18} strokeWidth={1.8} />
      </button>
    </form>
  );
}
