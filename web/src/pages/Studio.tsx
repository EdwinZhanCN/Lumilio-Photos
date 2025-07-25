import React, { useState, useRef, useEffect, useCallback } from "react";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata.tsx";
import {
  useGenerateBorders,
  BorderOptions,
  BorderParams,
} from "@/hooks/util-hooks/useGenerateBorder";
import { StudioHeader } from "@/components/Studio/StudioHeader";
import { StudioSidebar } from "@/components/Studio/StudioSidebar";
import { StudioViewport } from "@/components/Studio/StudioViewport";
import { StudioToolsPanel } from "@/components/Studio/StudioToolsPanel";

export type PanelType = "exif" | "develop" | "frames";

export function Studio() {
  // Core State
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [isCancelling, setIsCancelling] = useState<boolean>(false);
  const [isCancellingExif, setIsCancellingExif] = useState<boolean>(false);

  // UI State
  const [activePanel, setActivePanel] = useState<PanelType>("exif");
  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(false);

  const fileInputRef = useRef<HTMLInputElement>(null);

  // Processed EXIF data for display
  const [exifToDisplay, setExifToDisplay] = useState<Record<
    string,
    any
  > | null>(null);

  // Instantiate Hooks
  const {
    isExtracting,
    exifData,
    progress: exifProgress,
    extractExifData,
    cancelExtraction,
  } = useExtractExifdata();

  const {
    isGenerating: isGenBorders,
    processedImages,
    setProcessedImages,
    generateBorders,
    progress: borderProgress,
    cancelGeneration,
  } = useGenerateBorders();

  // Process raw EXIF data when it changes
  useEffect(() => {
    if (exifData && exifData[0]) {
      setExifToDisplay(exifData[0]);
    } else {
      setExifToDisplay(null);
    }
  }, [exifData]);

  //#region Handlers
  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      setSelectedFile(file);
      // Revoke previous URL to prevent memory leaks
      if (imageUrl) URL.revokeObjectURL(imageUrl);
      const newUrl = URL.createObjectURL(file);
      setImageUrl(newUrl);

      // Reset all states
      setExifToDisplay(null);
      setProcessedImages({});
      setActivePanel("exif");
    }
  };

  const handleExtractExif = useCallback(async () => {
    if (selectedFile) {
      setExifToDisplay(null);
      await extractExifData([selectedFile]);
      setActivePanel("exif");
    }
  }, [selectedFile, extractExifData]);

  const handleGenerateBorders = useCallback(
    async (option: BorderOptions, param: BorderParams[BorderOptions]) => {
      if (selectedFile) {
        await generateBorders([selectedFile], option, param);
      }
    },
    [selectedFile, generateBorders],
  );

  const handleCancelGeneration = useCallback(() => {
    setIsCancelling(true);
    try {
      cancelGeneration();
    } finally {
      setIsCancelling(false);
    }
  }, [cancelGeneration]);

  const handleCancelExtraction = useCallback(() => {
    setIsCancellingExif(true);
    try {
      cancelExtraction();
    } finally {
      setIsCancellingExif(false);
    }
  }, [cancelExtraction]);
  //#endregion

  const triggerFileInput = () => {
    fileInputRef.current?.click();
  };

  // Get the first processed image URL for display in the viewport
  const displayUrl =
    Object.values(processedImages).find((img) => img.borderedFileURL)
      ?.borderedFileURL || imageUrl;

  return (
    <div className="flex flex-col h-[calc(85vh)] bg-base-100 overflow-hidden">
      <StudioHeader
        onOpenFile={triggerFileInput}
        fileInputRef={fileInputRef}
        onFileChange={handleFileChange}
      />

      <div className="flex-1 flex overflow-hidden">
        <StudioSidebar
          activePanel={activePanel}
          setActivePanel={setActivePanel}
          isCollapsed={sidebarCollapsed}
          onToggle={() => setSidebarCollapsed(!sidebarCollapsed)}
        />

        <StudioViewport
          key={displayUrl} // Force re-render when URL changes
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
          isGeneratingBorders={isGenBorders}
          borderProgress={borderProgress}
          onGenerateBorders={handleGenerateBorders}
          onCancelGeneration={handleCancelGeneration}
          isCancelling={isCancelling}
          onCancelExtraction={handleCancelExtraction}
          isCancellingExif={isCancellingExif}
        />
      </div>
    </div>
  );
}
