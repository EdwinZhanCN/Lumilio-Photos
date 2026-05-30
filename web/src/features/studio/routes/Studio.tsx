import React, { useState, useRef, useEffect, useCallback } from "react";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata.tsx";
import { useWorker } from "@/contexts/WorkerProvider.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage.tsx";
import {
  StudioHeader,
  StudioSidebar,
  StudioToolsPanel,
  StudioViewport,
} from "@/features/studio/components";
import { DEFAULT_PARAMS } from "@/features/studio/tools/border";

type ToolProgress = {
  processed: number;
  total: number;
  error?: string;
  failedAt?: number | null;
} | null;

export type PanelType = "exif" | "develop" | "border";

export function Studio() {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [isCancellingExif, setIsCancellingExif] = useState(false);

  const [activePanel, setActivePanel] = useState<PanelType>("exif");
  const fileInputRef = useRef<HTMLInputElement>(null);

  const [exifToDisplay, setExifToDisplay] = useState<Record<string, any> | null>(null);

  // Tool state
  const [toolParams, setToolParams] = useState<Record<string, unknown>>(DEFAULT_PARAMS);
  const [toolResultUrl, setToolResultUrl] = useState<string | null>(null);
  const [toolResultFileName, setToolResultFileName] = useState<string | null>(null);
  const [isGeneratingTool, setIsGeneratingTool] = useState(false);
  const [toolProgress, setToolProgress] = useState<ToolProgress>(null);
  const [isCancellingTool, setIsCancellingTool] = useState(false);

  const workerClient = useWorker();
  const showMessage = useMessage();

  const {
    isExtracting,
    exifData,
    progress: exifProgress,
    extractExifData,
    cancelExtraction,
  } = useExtractExifdata();

  useEffect(() => {
    if (exifData && exifData[0]) {
      setExifToDisplay(exifData[0]);
    } else {
      setExifToDisplay(null);
    }
  }, [exifData]);

  useEffect(() => {
    const removeListener = workerClient.addProgressListener((detail) => {
      if (detail?.operation !== "tool") {
        return;
      }

      setToolProgress({
        processed: Number(detail.processed || 0),
        total: Number(detail.total || 100),
      });
    });

    return () => {
      removeListener();
    };
  }, [workerClient]);

  useEffect(() => {
    return () => {
      if (imageUrl) {
        URL.revokeObjectURL(imageUrl);
      }
    };
  }, [imageUrl]);

  useEffect(() => {
    return () => {
      if (toolResultUrl) {
        URL.revokeObjectURL(toolResultUrl);
      }
    };
  }, [toolResultUrl]);

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (!file) {
      return;
    }

    setSelectedFile(file);

    if (imageUrl) {
      URL.revokeObjectURL(imageUrl);
    }

    setToolResultUrl((prev) => {
      if (prev) {
        URL.revokeObjectURL(prev);
      }
      return null;
    });
    setToolResultFileName(null);

    const newUrl = URL.createObjectURL(file);
    setImageUrl(newUrl);

    setExifToDisplay(null);
    setToolProgress(null);
    setActivePanel("exif");
  };

  const handleExtractExif = useCallback(async () => {
    if (!selectedFile) {
      return;
    }

    setExifToDisplay(null);
    await extractExifData([selectedFile]);
    setActivePanel("exif");
  }, [selectedFile, extractExifData]);

  const handleGenerateTool = useCallback(async () => {
    if (!selectedFile) {
      return;
    }

    setIsGeneratingTool(true);
    setToolProgress({ processed: 0, total: 100 });

    try {
      const result = await workerClient.runTool(
        "border",
        selectedFile,
        toolParams,
      );

      const resultUrl = URL.createObjectURL(result.blob);
      setToolResultUrl((prev) => {
        if (prev) {
          URL.revokeObjectURL(prev);
        }
        return resultUrl;
      });
      setToolResultFileName(result.fileName);

      showMessage("success", `Processing complete: ${result.fileName}`);
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "Tool processing failed";
      setToolProgress((prev) =>
        prev
          ? {
              ...prev,
              error: message,
              failedAt: prev.processed,
            }
          : null,
      );
      showMessage("error", message);
    } finally {
      setIsGeneratingTool(false);
    }
  }, [selectedFile, toolParams, workerClient, showMessage]);

  const handleCancelToolGeneration = useCallback(() => {
    setIsCancellingTool(true);
    try {
      workerClient.abortTool();
      setIsGeneratingTool(false);
      setToolProgress(null);
      showMessage("info", "Tool generation has been cancelled.");
    } finally {
      setIsCancellingTool(false);
    }
  }, [workerClient, showMessage]);

  const handleCancelExtraction = useCallback(() => {
    setIsCancellingExif(true);
    try {
      cancelExtraction();
    } finally {
      setIsCancellingExif(false);
    }
  }, [cancelExtraction]);

  const handleExportToolResult = useCallback(() => {
    if (!toolResultUrl) {
      return;
    }

    try {
      const link = document.createElement("a");
      link.href = toolResultUrl;
      link.download = toolResultFileName || "tool-output.png";
      document.body.appendChild(link);
      link.click();
      link.remove();
    } catch {
      showMessage("error", "Failed to export output image");
    }
  }, [toolResultUrl, toolResultFileName, showMessage]);

  const triggerFileInput = () => {
    fileInputRef.current?.click();
  };

  const displayUrl = toolResultUrl || imageUrl;

  return (
    <div className="flex flex-col h-[calc(85vh)] bg-base-100 overflow-hidden">
      <StudioHeader
        onOpenFile={triggerFileInput}
        onExportImage={handleExportToolResult}
        hasExportImage={Boolean(toolResultUrl)}
        fileInputRef={fileInputRef}
        onFileChange={handleFileChange}
      />

      <div className="flex-1 flex overflow-hidden">
        <StudioSidebar
          activePanel={activePanel}
          setActivePanel={setActivePanel}
        />

        <StudioViewport
          key={displayUrl || "empty"}
          imageUrl={displayUrl}
          selectedFile={selectedFile}
          onOpenFile={triggerFileInput}
        />

        <StudioToolsPanel
          selectedFile={selectedFile}
          activePanel={activePanel}
          isExtracting={isExtracting}
          exifProgress={exifProgress}
          exifToDisplay={exifToDisplay}
          onExtractExif={handleExtractExif}
          isGeneratingTool={isGeneratingTool}
          toolProgress={toolProgress}
          onGenerateTool={handleGenerateTool}
          toolParams={toolParams}
          onToolParamsChange={setToolParams}
          onCancelToolGeneration={handleCancelToolGeneration}
          isCancellingTool={isCancellingTool}
          onCancelExtraction={handleCancelExtraction}
          isCancellingExif={isCancellingExif}
        />
      </div>
    </div>
  );
}
