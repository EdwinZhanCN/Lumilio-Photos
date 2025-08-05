import { createContext, useContext, useState, useEffect } from "react";
import { ModelRecord } from "@mlc-ai/web-llm";

// Default Qwen models
export const defaultModelRecords: ModelRecord[] = [
  {
    model: "https://huggingface.co/mlc-ai/Qwen3-4B-q4f16_1-MLC",
    model_id: "Qwen3-4B-q4f16_1-MLC",
    model_lib:
      "https://raw.githubusercontent.com/mlc-ai/binary-mlc-llm-libs/main/web-llm-models/v0_2_48/Qwen3-4B-q4f16_1-ctx4k_cs1k-webgpu.wasm",
    vram_required_MB: 3431.59,
    low_resource_required: true,
    overrides: {
      context_window_size: 4096,
    },
  },
  {
    model: "https://huggingface.co/mlc-ai/Qwen3-1.7B-q4f16_1-MLC",
    model_id: "Qwen3-1.7B-q4f16_1-MLC",
    model_lib:
      "https://raw.githubusercontent.com/mlc-ai/binary-mlc-llm-libs/main/web-llm-models/v0_2_48/Qwen3-1.7B-q4f16_1-ctx4k_cs1k-webgpu.wasm",
    vram_required_MB: 2036.66,
    low_resource_required: true,
    overrides: {
      context_window_size: 4096,
    },
  },
];

type Settings = {
  lumen?: {
    model: string;
    temperature: number;
    top_p: number;
    modelRecords?: ModelRecord[];
    systemPrompt?: string;
    enabled?: boolean;
  };
};

type SettingsContextType = {
  settings: Settings;
  setSettings: (settings: Settings) => void;
  isMobile: boolean;
};

export const SettingsContext = createContext<SettingsContextType | undefined>(
  undefined,
);

export const SettingsProvider = ({
  children,
}: {
  children: React.ReactNode;
}) => {
  const [isMobile, setIsMobile] = useState(false);

  // Check if device is mobile
  useEffect(() => {
    const checkIfMobile = () => {
      const isMobileDevice =
        /Android|webOS|iPhone|iPad|iPod|BlackBerry|IEMobile|Opera Mini/i.test(
          navigator.userAgent,
        ) || window.innerWidth <= 768;
      setIsMobile(isMobileDevice);
    };

    checkIfMobile();
    window.addEventListener("resize", checkIfMobile);

    return () => window.removeEventListener("resize", checkIfMobile);
  }, []);

  const [settings, setSettings] = useState<Settings>({
    lumen: {
      model: "Qwen3-1.7B-q4f16_1-MLC",
      temperature: 0.8,
      top_p: 0.95,
      modelRecords: defaultModelRecords,
      systemPrompt:
        "You are a helpful AI assistant that provides informative and concise responses about various topics. Be friendly and engaging in your responses.",
      enabled: true,
    },
  });

  // Override enabled state based on mobile detection
  const effectiveSettings = {
    ...settings,
    lumen: settings.lumen
      ? {
          ...settings.lumen,
          enabled: isMobile ? false : (settings.lumen.enabled ?? true),
        }
      : undefined,
  };

  return (
    <SettingsContext.Provider
      value={{ settings: effectiveSettings, setSettings, isMobile }}
    >
      {children}
    </SettingsContext.Provider>
  );
};

export const useSettings = () => {
  const context = useContext(SettingsContext);
  if (context === undefined) {
    throw new Error("useSettings must be used within a SettingsProvider");
  }
  return context;
};
