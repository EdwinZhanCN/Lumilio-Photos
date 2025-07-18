import React, { useState, useRef, useEffect, useCallback } from "react";
import { WasmWorkerClient, WorkerClient } from "@/workers/workerClient.ts";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata.tsx";
import {
  useGenerateBorders,
  BorderOptions,
  BorderParams,
} from "@/hooks/wasm-hooks/useGenerateBorder";
import { StudioHeader } from "@/components/Studio/StudioHeader";
import { StudioSidebar } from "@/components/Studio/StudioSidebar";
import { StudioViewport } from "@/components/Studio/StudioViewport";
import { StudioToolsPanel } from "@/components/Studio/StudioToolsPanel";

export type PanelType = "exif" | "develop" | "frames";

export function Studio() {
  // Core State
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);

  // UI State
  const [activePanel, setActivePanel] = useState<PanelType>("exif");
  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(false);
  const [wasmReady, setWasmReady] = useState(false);

  // Worker Refs
  const wasmWorkerClientRef = useRef<WasmWorkerClient | null>(null);
  const workerClientRef = useRef<WorkerClient | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Processed EXIF data for display
  const [exifToDisplay, setExifToDisplay] = useState<Record<
    string,
    any
  > | null>(null);

  // Initialize WorkerClient and WASM modules
  useEffect(() => {
    wasmWorkerClientRef.current = new WasmWorkerClient();
    wasmWorkerClientRef.current
      .initBorderWASM()
      .then(() => setWasmReady(true))
      .catch(console.error);

    workerClientRef.current = new WorkerClient();

    return () => {
      wasmWorkerClientRef.current?.terminateGenerateBorderWorker();
      workerClientRef.current?.terminateExtractExifWorker();
    };
  }, []);

  // Instantiate Hooks
  const {
    isExtracting,
    exifData,
    progress: exifProgress,
    extractExifData,
  } = useExtractExifdata({
    workerClientRef,
  });

  const {
    isGenerating: isGenBorders,
    progress: genBordersProgress,
    processedImages,
    setProcessedImages,
    generateBorders,
  } = useGenerateBorders({
    workerClientRef: wasmWorkerClientRef,
    wasmReady,
  });

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
          borderProgress={genBordersProgress}
          onGenerateBorders={handleGenerateBorders}
        />
      </div>
    </div>
  );
}
