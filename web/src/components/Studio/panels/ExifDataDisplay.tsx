import { InformationCircleIcon } from "@heroicons/react/24/outline";

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
        const { data: _, ...rest } = rawExif;
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
  const entries = processExifData(exifData);

  if (isLoading && !exifData) {
    return (
      <div className="text-center py-8">
        <span className="loading loading-lg loading-spinner text-primary"></span>
        <p className="mt-2">Extracting data...</p>
      </div>
    );
  }

  return (
    <div>
      <div className="flex items-center mb-4">
        <InformationCircleIcon className="w-5 h-5 mr-2" />
        <h2 className="text-lg font-semibold">EXIF Metadata</h2>
      </div>

      {entries.length === 0 ? (
        <div className="text-center py-4 text-gray-500">
          No metadata to display.
        </div>
      ) : (
        <div className="rounded-lg bg-base-100 shadow-sm max-h-[calc(70vh)] overflow-y-auto">
          <div className="bg-base-300 p-2 rounded-t-lg flex items-center sticky top-0 z-10">
            <h3 className="text-sm font-bold text-base-content">
              Image Properties
            </h3>
            <span className="ml-2 text-xs opacity-60">
              ({entries.length} fields)
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
