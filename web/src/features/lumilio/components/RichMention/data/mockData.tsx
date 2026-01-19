import React, { useEffect, useState } from "react";
import { MentionEntity, MentionType } from "../types";
import agentService from "@/services/agentService";
import { Image, Tag, Camera, Circle, MapPin, Terminal } from "lucide-react";

// --- 图标组件 (Lucide React) ---
export const IconAlbum = () => <Image size={14} className="inline-block" />;

const IconTag = () => <Tag size={14} className="inline-block" />;

const IconCamera = () => <Camera size={14} className="inline-block" />;

const IconLens = () => <Circle size={14} className="inline-block" />;

const IconLocation = () => <MapPin size={14} className="inline-block" />;

export const IconCommand = () => (
  <Terminal size={14} className="inline-block" />
);

// --- 模拟数据 ---

export const MENTION_TYPES: {
  type: MentionType;
  label: string;
  icon: React.ReactNode;
  desc: string;
}[] = [
  {
    type: "album",
    label: "Album",
    icon: <IconAlbum />,
    desc: "Add to collection",
  },
  { type: "tag", label: "Tag", icon: <IconTag />, desc: "Attach tags" },
  {
    type: "camera",
    label: "Camera",
    icon: <IconCamera />,
    desc: "Filter by device",
  },
  { type: "lens", label: "Lens", icon: <IconLens />, desc: "Filter by lens" },
  {
    type: "location",
    label: "Location",
    icon: <IconLocation />,
    desc: "Geographic filter",
  },
];

// Hook to fetch agent tools and update COMMANDS
export function useAgentTools() {
  const [commands, setCommands] = useState<MentionEntity[]>([]);

  useEffect(() => {
    async function fetchTools() {
      try {
        const tools = await agentService.getSlashCommands();
        const commandEntities: MentionEntity[] = tools.map((tool) => ({
          id: tool.id,
          label: tool.label,
          type: "command" as const,
          meta: tool.meta,
          icon: <IconCommand />,
        }));
        setCommands(commandEntities);
      } catch (error) {
        console.error("Failed to fetch agent tools:", error);
      }
    }

    fetchTools();
  }, []);

  return commands;
}

export const MOCK_ENTITIES: Record<string, MentionEntity[]> = {
  album: [
    {
      id: "uuid-a1",
      label: "Summer Trip 2023",
      type: "album",
      meta: "50 photos",
    },
    {
      id: "uuid-a2",
      label: "Japan Vacation",
      type: "album",
      meta: "200 photos",
    },
    {
      id: "uuid-a3",
      label: "Family Reunion",
      type: "album",
      meta: "12 photos",
    },
    { id: "uuid-a4", label: "Project X", type: "album", meta: "Empty" },
  ],
  tag: [
    { id: "tag-1", label: "Landscape", type: "tag" },
    { id: "tag-2", label: "Portrait", type: "tag" },
    { id: "tag-3", label: "Macro", type: "tag" },
    { id: "tag-4", label: "Street", type: "tag" },
  ],
  camera: [
    { id: "cam-1", label: "Sony A7M3", type: "camera" },
    { id: "cam-2", label: "Fujifilm X100V", type: "camera" },
    { id: "cam-3", label: "iPhone 15 Pro", type: "camera" },
  ],
  lens: [
    { id: "lens-1", label: "FE 50mm F1.8", type: "lens" },
    { id: "lens-2", label: "FE 24-70mm GM", type: "lens" },
  ],
  location: [
    { id: "loc-1", label: "Tokyo, Japan", type: "location" },
    { id: "loc-2", label: "New York, USA", type: "location" },
    { id: "loc-3", label: "Paris, France", type: "location" },
  ],
};

// This will be populated with actual agent tools
export const COMMANDS: MentionEntity[] = [];
