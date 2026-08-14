import { code } from "@streamdown/code";
import { cjk } from "@streamdown/cjk";
import { createMathPlugin } from "@streamdown/math";
import { Streamdown, type Components } from "streamdown";
import { isAsyncClipboardAvailable } from "@/lib/clipboard";
import { Img, Link } from "./MarkdownBlocks";

const math = createMathPlugin({
  singleDollarTextMath: true,
  errorColor: "var(--color-error)",
});

const plugins = { code, cjk, math };

const components: Components = {
  img: Img,
  a: Link,
  p: (props) => <div {...props} />,
};

type MarkdownProps = {
  content?: string;
  className?: string;
  isAnimating?: boolean;
};

export function getMarkdownControls(
  canUseStreamdownCopy = isAsyncClipboardAvailable(),
): React.ComponentProps<typeof Streamdown>["controls"] {
  return {
    code: { copy: canUseStreamdownCopy, download: false },
    table: { copy: canUseStreamdownCopy, download: false, fullscreen: false },
  };
}

export const Markdown = ({
  content = "",
  className = "text-base leading-relaxed",
  isAnimating = false,
}: MarkdownProps) => {
  return (
    <Streamdown
      className={className}
      components={components}
      controls={getMarkdownControls()}
      dir="auto"
      isAnimating={isAnimating}
      lineNumbers={false}
      plugins={plugins}
    >
      {content}
    </Streamdown>
  );
};
