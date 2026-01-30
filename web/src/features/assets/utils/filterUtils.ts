
export function mapFilenameModeToDTO(
    mode?: "contains" | "matches" | "startswith" | "endswith",
): "contains" | "matches" | "starts_with" | "ends_with" | undefined {
    switch (mode) {
        case "startswith":
            return "starts_with";
        case "endswith":
            return "ends_with";
        case "contains":
        case "matches":
            return mode;
        default:
            return undefined;
    }
}

export function mapFilenameOperatorToMode(
    op?: "contains" | "matches" | "starts_with" | "ends_with",
): "contains" | "matches" | "startswith" | "endswith" | undefined {
    switch (op) {
        case "starts_with":
            return "startswith";
        case "ends_with":
            return "endswith";
        case "contains":
        case "matches":
            return op;
        default:
            return undefined;
    }
}
