// Simplified frame interface without complex configuration options
export interface PhotoFrame {
    // Unique identifier for the frame
    id: string;

    // Display name for the frame
    name: string;

    // Brief description of the frame
    description: string;

    // Author information
    author: string;

    // Version of the frame
    version: string;

    // Tags for categorization
    tags: string[];

    // Whether this frame supports metadata display
    supportsMetadata: boolean;
}

// Complete frame definition (simplified)
export interface FrameDefinition extends PhotoFrame {
    // The React component for this frame (no longer needed with simplified approach)
    component: React.ComponentType<any> | null;
}

// Frame category for organization
export interface FrameCategory {
    id: string;
    name: string;
    description: string;
    frames: FrameDefinition[];
}

// Export result from frame
export interface FrameExportResult {
    dataUrl: string;
    filename: string;
    metadata?: Record<string, any>;
}

// Legacy types kept for backward compatibility but not used in simplified version
export interface FrameOption {
    id: string;
    name: string;
    type: "text" | "number" | "boolean" | "color" | "select" | "range";
    defaultValue: any;
    description?: string;
    min?: number;
    max?: number;
    step?: number;
    options?: { value: any; label: string }[];
}

export interface FrameComponentProps {
    src: string;
    metadata?: string;
    config?: Record<string, any>;
    width?: number;
    height?: number;
    onExport?: (dataUrl: string, filename?: string) => void;
    onReady?: () => void;
    onConfigChange?: (config: Record<string, any>) => void;
}

export type FrameComponent = React.ComponentType<FrameComponentProps>;
