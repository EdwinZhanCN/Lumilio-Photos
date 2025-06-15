import { FrameDefinition } from "./types";

// Simplified frame definitions without complex configuration options
const frameDefinitions: FrameDefinition[] = [
    {
        id: "glass-frame",
        name: "Glass Frame",
        description:
            "A modern glassmorphism-style frame with frosted glass borders and elegant shadows.",
        author: "RKPhoto Team",
        version: "1.0.0",
        tags: ["modern", "glass", "elegant", "transparent"],
        supportsMetadata: true,
        component: null, // No longer needed with simplified approach
    },
    {
        id: "classic-frame",
        name: "Classic Frame",
        description:
            "A traditional wooden picture frame with ornate borders and vintage styling.",
        author: "RKPhoto Team",
        version: "1.0.0",
        tags: ["classic", "wooden", "traditional", "vintage"],
        supportsMetadata: true,
        component: null,
    },
    {
        id: "polaroid-frame",
        name: "Polaroid Frame",
        description:
            "A nostalgic instant camera photo style with white borders and handwritten text area.",
        author: "RKPhoto Team",
        version: "1.0.0",
        tags: ["polaroid", "instant", "vintage", "retro", "square"],
        supportsMetadata: true,
        component: null,
    },
];

// Simplified frame registry class
export class FrameRegistry {
    private static instance: FrameRegistry;
    private frames: Map<string, FrameDefinition> = new Map();

    private constructor() {
        this.loadFrames();
    }

    public static getInstance(): FrameRegistry {
        if (!FrameRegistry.instance) {
            FrameRegistry.instance = new FrameRegistry();
        }
        return FrameRegistry.instance;
    }

    private loadFrames(): void {
        frameDefinitions.forEach((frame) => {
            this.frames.set(frame.id, frame);
        });
    }

    public getAllFrames(): FrameDefinition[] {
        return Array.from(this.frames.values());
    }

    public getFrame(id: string): FrameDefinition | undefined {
        return this.frames.get(id);
    }

    public getFramesByTag(tag: string): FrameDefinition[] {
        return this.getAllFrames().filter((frame) =>
            frame.tags.includes(tag.toLowerCase()),
        );
    }

    public getFramesByAuthor(author: string): FrameDefinition[] {
        return this.getAllFrames().filter(
            (frame) => frame.author.toLowerCase() === author.toLowerCase(),
        );
    }

    public searchFrames(query: string): FrameDefinition[] {
        const searchTerm = query.toLowerCase();
        return this.getAllFrames().filter(
            (frame) =>
                frame.name.toLowerCase().includes(searchTerm) ||
                frame.description.toLowerCase().includes(searchTerm) ||
                frame.tags.some((tag) =>
                    tag.toLowerCase().includes(searchTerm),
                ) ||
                frame.author.toLowerCase().includes(searchTerm),
        );
    }

    public getFrameCategories(): { [category: string]: FrameDefinition[] } {
        const categories: { [category: string]: FrameDefinition[] } = {};

        this.getAllFrames().forEach((frame) => {
            frame.tags.forEach((tag) => {
                if (!categories[tag]) {
                    categories[tag] = [];
                }
                if (!categories[tag].includes(frame)) {
                    categories[tag].push(frame);
                }
            });
        });

        return categories;
    }

    public registerFrame(frame: FrameDefinition): void {
        if (this.frames.has(frame.id)) {
            console.warn(
                `Frame with id '${frame.id}' is already registered. Overwriting.`,
            );
        }
        this.frames.set(frame.id, frame);
    }

    public unregisterFrame(id: string): boolean {
        return this.frames.delete(id);
    }
}

// Export convenience functions
export const frameRegistry = FrameRegistry.getInstance();

export const getAllFrames = () => frameRegistry.getAllFrames();
export const getFrame = (id: string) => frameRegistry.getFrame(id);
export const getFramesByTag = (tag: string) =>
    frameRegistry.getFramesByTag(tag);
export const searchFrames = (query: string) =>
    frameRegistry.searchFrames(query);
export const getFrameCategories = () => frameRegistry.getFrameCategories();
