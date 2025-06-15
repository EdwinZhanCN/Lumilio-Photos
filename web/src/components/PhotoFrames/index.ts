// Export simplified frame types and interfaces
export type {
    PhotoFrame,
    FrameDefinition,
    FrameCategory,
    FrameExportResult,
} from "./types";

// Export the frame registry and related functions
export {
    FrameRegistry,
    frameRegistry,
    getAllFrames,
    getFrame,
    getFramesByTag,
    searchFrames,
    getFrameCategories,
} from "./frameRegistry";

// Export the simple frame picker component
export { SimpleFramePicker } from "./SimpleFramePicker";
