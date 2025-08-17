// useStreamingSplit.ts
import { useMemo, useRef } from "react";

export function useStreamingSplit(content: string, isStreaming: boolean) {
    const prevRef = useRef("");
    const { prefix, suffix } = useMemo(() => {
        if (!isStreaming) return { prefix: content, suffix: "" };

        const prev = prevRef.current || "";
        // 最长公共前缀
        let i = 0;
        const len = Math.min(prev.length, content.length);
        while (i < len && prev[i] === content[i]) i++;

        return {
            prefix: content.slice(0, i),
            suffix: content.slice(i),
        };
    }, [content, isStreaming]);

    // 记录上次（只在流式时记录，避免回退时抖动）
    if (isStreaming) prevRef.current = content;

    return { prefix, suffix };
}