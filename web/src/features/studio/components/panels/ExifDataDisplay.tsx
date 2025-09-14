import {
  InformationCircleIcon,
  ArrowDownTrayIcon,
} from "@heroicons/react/24/outline";
import { useI18n } from "@/lib/i18n.tsx";

type ExifDataDisplayProps = {
  exifData: Record<string, any> | null;
  isLoading: boolean;
};

// Helper function to format EXIF values for better readability
const formatExifValue = (key: string, value: any): string => {
  if (value === null || value === undefined || value === "") return "N/A";
  if (typeof value === "string" && value.trim() === "") return "N/A";

  if (key.toLowerCase().includes("date") && !isNaN(new Date(value).getTime())) {
    return new Date(value).toLocaleString();
  }
  if (
    key.toLowerCase().includes("exposuretime") &&
    typeof value === "number" &&
    value > 0 &&
    value < 1
  ) {
    return `1/${Math.round(1 / value)}`;
  }
  if (typeof value === "number") {
    return Number.isInteger(value) ? value.toString() : value.toFixed(2);
  }
  if (typeof value === "object") {
    return JSON.stringify(value, null, 2);
  }
  return String(value);
};

// Function to process and clean the raw EXIF data object
const processExifData = (
  rawExif: Record<string, any> | any[] | null,
): [string, any][] => {
  // 1. Handle null or empty input immediately
  if (!rawExif || (Array.isArray(rawExif) && rawExif.length === 0)) {
    return [];
  }

  let dataToProcess: Record<string, any> = {};

  // 2. Determine the actual data object to process in a type-safe way
  if (Array.isArray(rawExif)) {
    // If the input is an array, we assume the first element is our data
    dataToProcess = rawExif[0] ?? {};
  } else if (typeof rawExif === "object" && rawExif !== null) {
    // If it's an object, check for the special 'data' property
    if ("data" in rawExif && rawExif.data) {
      try {
        // Handle cases where 'data' might be a JSON string
        const parsed =
          typeof rawExif.data === "string"
            ? JSON.parse(rawExif.data)
            : rawExif.data;

        // The parsed data could also be an array
        dataToProcess =
          Array.isArray(parsed) && parsed.length > 0 ? parsed[0] : parsed;
      } catch (e) {
        // If parsing fails, use the parent object but exclude the problematic 'data' field
        const rest = { ...(rawExif as Record<string, any>) };
        delete rest.data;
        dataToProcess = rest;
        throw new Error(`Failed to parse EXIF data: ${e}`);
      }
    } else {
      // It's a regular object without a 'data' property
      dataToProcess = rawExif;
    }
  }

  // 3. Filter and sort the final data object's entries
  const excludedKeys = [
    "success",
    "error",
    "exitcode",
    "sourcefile",
    "directory",
  ];

  if (typeof dataToProcess !== "object" || dataToProcess === null) {
    return [];
  }

  return Object.entries(dataToProcess)
    .filter(
      ([key]) =>
        !excludedKeys.some((exKey) => key.toLowerCase().includes(exKey)),
    )
    .sort((a, b) => a[0].localeCompare(b[0]));
};

export function ExifDataDisplay({ exifData, isLoading }: ExifDataDisplayProps) {
  const { t } = useI18n();
  const entries = processExifData(exifData);

  const handleDownload = () => {
    try {
      const data = exifData ?? {};
      const blob = new Blob([JSON.stringify(data, null, 2)], {
        type: "application/json",
      });
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = "exif.json";
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch {
      // Non-fatal: ignore download errors
    }
  };

  if (isLoading && !exifData) {
    return (
      <div className="text-center py-8">
        <span className="loading loading-lg loading-spinner text-primary"></span>
        <p className="mt-2">{t("studio.exif.extracting")}</p>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-4">
        <div className="flex items-center">
          <InformationCircleIcon className="w-5 h-5 mr-2" />
          <h2 className="text-lg font-semibold">{t("studio.exif.title")}</h2>
        </div>
        <button
          type="button"
          className="btn btn-ghost btn-sm"
          onClick={handleDownload}
          disabled={isLoading || !exifData}
          title="Download JSON"
          aria-label="Download JSON"
        >
          <ArrowDownTrayIcon className="w-5 h-5" />
        </button>
      </div>

      {entries.length === 0 ? (
        <div className="text-center py-4 text-gray-500">
          {t("studio.exif.empty")}
        </div>
      ) : (
        <div className="rounded-lg bg-base-100 shadow-sm max-h-[calc(70vh)] overflow-y-auto">
          <div className="bg-base-300 p-2 rounded-t-lg flex items-center sticky top-0 z-10">
            <h3 className="text-sm font-bold text-base-content">
              {t("studio.exif.properties")}
            </h3>
            <span className="ml-2 text-xs opacity-60">
              {t("studio.exif.fieldsCount", { count: entries.length })}
            </span>
          </div>
          <div className="divide-y divide-base-300/20">
            {entries.map(([key, value]) => (
              <div
                key={key}
                className="flex justify-between py-2 px-3 hover:bg-base-200/40 transition-colors"
              >
                <div className="font-medium text-xs text-base-content/80 w-2/5 mr-2 truncate">
                  {key}
                </div>
                <div className="text-xs flex-1 break-words font-mono text-right">
                  {formatExifValue(key, value)}
                </div>
              </div>
            ))}
          </div>
        </div>
      )}
    </div>
  );
}
