import { code } from "@streamdown/code";
import { cjk } from "@streamdown/cjk";
import { createMathPlugin } from "@streamdown/math";
import { Streamdown, type Components } from "streamdown";
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

interface MarkdownProps {
  content?: string;
  className?: string;
  isAnimating?: boolean;
}

export const Markdown = ({
  content = "",
  className = "text-base leading-relaxed",
  isAnimating = false,
}: MarkdownProps) => (
  <Streamdown
    className={className}
    components={components}
    controls={{
      code: { copy: true, download: false },
      table: { copy: true, download: false, fullscreen: false },
    }}
    dir="auto"
    isAnimating={isAnimating}
    lineNumbers={false}
    plugins={plugins}
  >
    {content}
  </Streamdown>
);
