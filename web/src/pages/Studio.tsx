import React, { useState, useRef, useEffect, useCallback } from "react";
import { WorkerClient } from "@/workers/workerClient.ts";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata.tsx";
import { SimpleFramePicker } from "@/components/PhotoFrames/SimpleFramePicker";
import {
  ArrowUpTrayIcon,
  DocumentTextIcon,
  ExclamationCircleIcon,
  AdjustmentsHorizontalIcon,
  PhotoIcon,
  Cog6ToothIcon,
  InformationCircleIcon,
  ArrowLeftIcon,
  ArrowRightIcon,
  PaintBrushIcon,
  RectangleGroupIcon,
} from "@heroicons/react/24/outline";

export function Studio() {
  const [selectedFile, setSelectedFile] = useState<File | null>(null);
  const [imageUrl, setImageUrl] = useState<string | null>(null);
  const [framedImageUrl, setFramedImageUrl] = useState<string | null>(null);
  const [activePanel, setActivePanel] = useState<"exif" | "develop" | "frames">(
    "exif",
  );
  const [sidebarCollapsed, setSidebarCollapsed] = useState<boolean>(false);

  // State for the hook to populate (Record<number, any>)
  const [rawExifDataFromHook, setRawExifDataFromHook] = useState<Record<
    number,
    any
  > | null>(null);
  // State for the single image's EXIF data to display (Record<string, any>)
  const [exifToDisplay, setExifToDisplay] = useState<Record<
    string,
    any
  > | null>(null);

  const [isExtracting, setIsExtracting] = useState(false);
  const [progress, setProgress] = useState<{
    numberProcessed: number;
    total: number;
    error?: string;
    failedAt?: number | null;
  } | null>(null);

  const workerClientRef = useRef<WorkerClient | null>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);

  // Initialize WorkerClient
  useEffect(() => {
    workerClientRef.current = new WorkerClient();
    return () => {
      workerClientRef.current?.terminateExtractExifWorker();
    };
  }, []);

  // Instantiate the hook
  const { extractExifData } = useExtractExifdata({
    workerClientRef,
    setExtractExifProgress: setProgress,
    setIsExtractingExif: setIsExtracting,
    setExifData: setRawExifDataFromHook, // Hook updates this state
  });

  // Process raw EXIF data from the hook when it changes
  useEffect(() => {
    if (rawExifDataFromHook && rawExifDataFromHook[0]) {
      // Assuming the first item (index 0) is our single image's data
      setExifToDisplay(rawExifDataFromHook[0]);
    } else {
      setExifToDisplay(null);
    }
  }, [rawExifDataFromHook]);

  const handleFileChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    if (file) {
      setSelectedFile(file);
      const reader = new FileReader();
      reader.onloadend = () => {
        setImageUrl(reader.result as string);
      };
      reader.readAsDataURL(file);
      setExifToDisplay(null); // Reset previous EXIF data
      setProgress(null); // Reset progress
      setRawExifDataFromHook(null); // Reset raw data
      setFramedImageUrl(null); // Reset framed image
    }
  };

  const handleExtractExif = useCallback(async () => {
    if (selectedFile && workerClientRef.current) {
      setExifToDisplay(null); // Clear previous results
      await extractExifData([selectedFile]);
      setActivePanel("exif");
    }
  }, [selectedFile, extractExifData]);

  const triggerFileInput = () => {
    fileInputRef.current?.click();
  };

  const toggleSidebar = () => {
    setSidebarCollapsed(!sidebarCollapsed);
  };

  const renderExifData = () => {
    if (!exifToDisplay) return null;

    if (!exifToDisplay || Object.keys(exifToDisplay).length === 0) {
      return (
        <p className="text-center text-gray-500">
          No EXIF data found for this image.
        </p>
      );
    }

    // Extract the EXIF data from the array format returned by the hook
    let processedExifData: Record<string, any> = {};

    // Check if exifToDisplay is directly an array (some hook implementations)
    if (Array.isArray(exifToDisplay)) {
      if (exifToDisplay.length > 0 && typeof exifToDisplay[0] === "object") {
        processedExifData = exifToDisplay[0];
      }
    }
    // Check if exifToDisplay has a data field that's already parsed as an array
    else if (exifToDisplay.data && Array.isArray(exifToDisplay.data)) {
      if (
        exifToDisplay.data.length > 0 &&
        typeof exifToDisplay.data[0] === "object"
      ) {
        processedExifData = exifToDisplay.data[0];
      }
    }
    // Check if exifToDisplay has a data field that's a JSON string
    else if (exifToDisplay.data && typeof exifToDisplay.data === "string") {
      try {
        const parsed = JSON.parse(exifToDisplay.data);

        if (Array.isArray(parsed) && parsed.length > 0) {
          // Use the first object in the array directly
          processedExifData = parsed[0];
        } else if (typeof parsed === "object" && parsed !== null) {
          // Or use the parsed object directly
          processedExifData = parsed;
        }
      } catch (e) {
        // If parsing fails, use the original data without the data field
        const { data, ...restData } = exifToDisplay;
        processedExifData = restData;
      }
    }
    // Use the object directly if no special handling needed
    else {
      processedExifData = exifToDisplay;
    }

    // Helper function to format EXIF values more readable
    const formatExifValue = (key: string, value: any): string => {
      if (value === null || value === undefined) return "N/A";

      // Handle special cases for better readability
      if (
        key.toLowerCase().includes("date") ||
        key.toLowerCase().includes("time")
      ) {
        if (value instanceof Date || !isNaN(new Date(value).getTime())) {
          return new Date(value).toLocaleString();
        }
      } else if (
        key.toLowerCase().includes("gps") &&
        typeof value === "number"
      ) {
        return value.toFixed(6).toString();
      } else if (typeof value === "number") {
        // Handle fractions like exposure values (1/100)
        if (Math.abs(value) < 0.01 && value !== 0) {
          const denominator = Math.round(1 / value);
          return `1/${denominator}`;
        }
        // Format decimal numbers nicely
        return Number.isInteger(value) ? value.toString() : value.toFixed(2);
      } else if (typeof value === "object") {
        try {
          return JSON.stringify(value);
        } catch (e) {
          return "[Complex Object]";
        }
      }

      return String(value);
    };

    // Filter out system info keys
    const excludedKeys = ["success", "error", "exitcode"];

    // Get all entries, filter excluded keys, and sort alphabetically
    const entries = Object.entries(processedExifData)
      .filter(
        ([key]) =>
          !excludedKeys.some((excluded) =>
            key.toLowerCase().includes(excluded),
          ),
      )
      .sort((a, b) => a[0].localeCompare(b[0]));

    return (
      <div className="overflow-y-auto overflow-x-hidden max-h-[calc(60vh)] px-1">
        {entries.length === 0 ? (
          <div className="text-center py-4 text-gray-500">
            No metadata to display
          </div>
        ) : (
          <div className="rounded-lg bg-base-100 shadow-sm mb-4">
            <div className="bg-base-300 p-2 rounded-t-lg flex items-center">
              <h3 className="text-sm font-bold text-base-content">EXIF Data</h3>
              <span className="ml-2 text-xs opacity-60">
                ({entries.length} fields)
              </span>
            </div>
            <div className="divide-y divide-base-300/20">
              {entries.map(([key, value]) => (
                <div
                  key={key}
                  className="flex py-2 px-3 hover:bg-base-200/40 transition-colors"
                >
                  <div className="font-medium text-xs text-base-content/80 w-1/2 mr-2">
                    {key}
                  </div>
                  <div className="text-xs flex-1 break-all font-mono">
                    {formatExifValue(key, value)}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>
    );
  };

  // Placeholder for future panels
  const renderDevelopPanel = () => (
    <div className="p-4 bg-base-300 rounded-lg h-full min-h-[300px] flex items-center justify-center">
      <div className="text-center">
        <AdjustmentsHorizontalIcon className="w-12 h-12 mx-auto text-base-content/50" />
        <h3 className="mt-2 text-lg font-semibold">Development Tools</h3>
        <p className="mt-1 text-sm text-base-content/70">Coming soon...</p>
      </div>
    </div>
  );

  const handleFrameExport = useCallback((dataUrl: string, filename: string) => {
    // Create download link
    const link = document.createElement("a");
    link.download = filename;
    link.href = dataUrl;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
  }, []);

  const handleFramedImageChange = useCallback(
    (newFramedImageUrl: string | null) => {
      setFramedImageUrl(newFramedImageUrl);
    },
    [],
  );

  const renderFramesPanel = () => {
    // Generate metadata string from EXIF data
    let metadataString = "";
    if (exifToDisplay && selectedFile) {
      const date = exifToDisplay.DateTime || exifToDisplay.DateTimeOriginal;
      const camera =
        exifToDisplay.Make && exifToDisplay.Model
          ? `${exifToDisplay.Make} ${exifToDisplay.Model}`
          : null;
      const settings = [];

      if (exifToDisplay.FNumber) settings.push(`f/${exifToDisplay.FNumber}`);
      if (exifToDisplay.ExposureTime)
        settings.push(`${exifToDisplay.ExposureTime}s`);
      if (exifToDisplay.ISOSpeedRatings)
        settings.push(`ISO ${exifToDisplay.ISOSpeedRatings}`);

      const parts = [];
      if (date) parts.push(new Date(date).toLocaleDateString());
      if (camera) parts.push(camera);
      if (settings.length > 0) parts.push(settings.join(", "));

      metadataString = parts.join(" • ");
    }

    return (
      <div className="h-full">
        <div className="flex items-center mb-4">
          <RectangleGroupIcon className="w-5 h-5 mr-2" />
          <h2 className="text-lg font-semibold">Photo Frames</h2>
        </div>
        <SimpleFramePicker
          imageUrl={imageUrl}
          metadata={metadataString}
          onFramedImageChange={handleFramedImageChange}
          onExport={handleFrameExport}
        />
      </div>
    );
  };

  const renderCurrentPanel = () => {
    switch (activePanel) {
      case "exif":
        return (
          <div className="h-full">
            <div className="flex items-center mb-4">
              <InformationCircleIcon className="w-5 h-5 mr-2" />
              <h2 className="text-lg font-semibold">EXIF Metadata</h2>
            </div>
            {isExtracting && !exifToDisplay && !progress?.error ? (
              <div className="text-center py-8">
                <span className="loading loading-lg loading-spinner text-primary"></span>
                <p className="mt-2">Extracting data...</p>
              </div>
            ) : (
              renderExifData()
            )}
          </div>
        );
      case "develop":
        return renderDevelopPanel();
      case "frames":
        return renderFramesPanel();
      default:
        return null;
    }
  };

  return (
    <div className="flex flex-col h-[calc(85vh)]  bg-base-100 overflow-hidden">
      {/* Top Toolbar */}
      <header className="py-2 px-4 border-b border-base-content/10 flex justify-between items-center">
        <div className="flex items-center space-x-2">
          <PaintBrushIcon className="w-6 h-6 text-primary" />
          <h1 className="text-xl font-bold">Studio</h1>
        </div>
        <div className="flex items-center space-x-2">
          <input
            type="file"
            ref={fileInputRef}
            onChange={handleFileChange}
            accept="image/jpeg, image/png, image/tiff, image.heic, image.heif, image.webp"
            className="hidden"
          />
          <button onClick={triggerFileInput} className="btn btn-sm btn-primary">
            <ArrowUpTrayIcon className="w-4 h-4 mr-1" />
            Open Image
          </button>
        </div>
      </header>

      {/* Main Content */}
      <div className="flex-1 flex overflow-hidden">
        {/* Left Sidebar - Modules */}
        <div
          className={`bg-base-200 border-r border-base-content/10 flex flex-col ${sidebarCollapsed ? "w-14" : "w-44"} transition-all duration-300`}
        >
          <div className="p-2">
            <button
              className="btn btn-sm btn-ghost w-full justify-center"
              onClick={toggleSidebar}
            >
              {sidebarCollapsed ? (
                <ArrowRightIcon className="w-5 h-5 flex-shrink-0" />
              ) : (
                <ArrowLeftIcon className="w-5 h-5 flex-shrink-0" />
              )}
            </button>
          </div>
          <div className="p-2">
            <button
              className={`btn btn-sm w-full mb-2 ${activePanel === "exif" ? "btn-primary" : "btn-ghost"} ${sidebarCollapsed ? "justify-center px-0" : "justify-start"}`}
              onClick={() => setActivePanel("exif")}
            >
              <DocumentTextIcon className="w-5 h-5 flex-shrink-0" />
              {!sidebarCollapsed && <span className="ml-1">EXIF</span>}
            </button>
            <button
              className={`btn btn-sm w-full mb-2 ${activePanel === "develop" ? "btn-primary" : "btn-ghost"} ${sidebarCollapsed ? "justify-center px-0" : "justify-start"}`}
              onClick={() => setActivePanel("develop")}
            >
              <AdjustmentsHorizontalIcon className="w-5 h-5 flex-shrink-0" />
              {!sidebarCollapsed && <span className="ml-1">Develop</span>}
            </button>
            <button
              className={`btn btn-sm w-full ${activePanel === "frames" ? "btn-primary" : "btn-ghost"} ${sidebarCollapsed ? "justify-center px-0" : "justify-start"}`}
              onClick={() => setActivePanel("frames")}
            >
              <RectangleGroupIcon className="w-5 h-5 flex-shrink-0" />
              {!sidebarCollapsed && <span className="ml-1">Frames</span>}
            </button>
          </div>
        </div>

        {/* Main Editor Area */}
        {imageUrl ? (
          <div className="flex-1 overflow-hidden bg-base-300 relative">
            <div className="absolute inset-0 flex items-center justify-center p-4">
              <img
                src={framedImageUrl || imageUrl}
                alt={selectedFile?.name || "Preview"}
                className={`object-contain ${framedImageUrl ? "max-h-[70vh] max-w-[80%]" : "max-h-full max-w-full"}`}
              />
            </div>
            {/* Image Info Overlay */}
            <div className="absolute bottom-0 left-0 right-0 bg-base-300/80 backdrop-blur-sm p-2 text-xs">
              {selectedFile && (
                <>
                  <span className="font-semibold mr-2">
                    {selectedFile.name}
                  </span>
                  <span>{(selectedFile.size / 1024).toFixed(2)} KB</span>
                  {framedImageUrl && (
                    <span className="ml-2 px-2 py-0.5 bg-primary/20 text-primary rounded text-xs">
                      Framed
                    </span>
                  )}
                </>
              )}
            </div>
          </div>
        ) : (
          <div className="flex-1 flex items-center justify-center bg-base-300">
            <div className="text-center p-8">
              <PhotoIcon className="w-16 h-16 mx-auto text-base-content/30" />
              <p className="mt-4">No image selected</p>
              <button
                onClick={triggerFileInput}
                className="btn btn-primary mt-4"
              >
                Open an Image
              </button>
            </div>
          </div>
        )}
        {/* Right Panel - Tools and Properties */}
        <div className="bg-base-200 border-l border-base-content/10 w-80 overflow-y-auto">
          {selectedFile ? (
            <div className="p-4">
              {!exifToDisplay && !isExtracting && (
                <div className="mb-4">
                  <button
                    onClick={handleExtractExif}
                    className="btn btn-secondary w-full"
                    disabled={isExtracting}
                  >
                    {isExtracting ? "Extracting..." : "Extract Metadata"}
                  </button>
                </div>
              )}

              {isExtracting && progress && (
                <div className="mb-4">
                  <p className="text-sm text-center mb-1">
                    Processing: {progress.numberProcessed} / {progress.total}
                  </p>
                  <progress
                    className="progress progress-primary w-full"
                    value={progress.numberProcessed}
                    max={progress.total}
                  ></progress>
                </div>
              )}

              {progress?.error && (
                <div role="alert" className="alert alert-error mb-4">
                  <ExclamationCircleIcon className="w-5 h-5" />
                  <div className="text-xs">{progress.error}</div>
                </div>
              )}

              {/* Render current panel content */}
              {renderCurrentPanel()}
            </div>
          ) : (
            <div className="p-4 text-center text-base-content/70">
              <p>Select an image to view its properties and apply edits</p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
