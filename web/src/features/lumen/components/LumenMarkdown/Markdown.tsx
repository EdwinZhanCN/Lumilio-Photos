// 只剩最小核心组件
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import { CodeBlock, Img, Link, ThinkBlock } from "./MarkdownBlocks";
import "@/styles/llm-token-fade.css"

// Create a custom sanitize schema that allows KaTeX elements
const mathSafeSchema = {
  ...defaultSchema,
  attributes: {
    ...defaultSchema.attributes,
    span: [
      ...(defaultSchema.attributes?.span || []),
      "className",
      "style",
      ["className", /^katex/],
    ],
    div: [
      ...(defaultSchema.attributes?.div || []),
      "className",
      "style",
      ["className", /^katex/],
    ],
    annotation: ["encoding"],
    math: ["display"],
    semantics: [],
    mrow: [],
    mo: [],
    mi: [],
    mn: [],
    mfrac: [],
    msup: [],
    msub: [],
    msubsup: [],
    munder: [],
    mover: [],
    munderover: [],
    mtable: [],
    mtr: [],
    mtd: [],
    mtext: [],
    mspace: ["width"],
  },
  tagNames: [
    ...(defaultSchema.tagNames || []),
    "math",
    "annotation",
    "semantics",
    "mrow",
    "mo",
    "mi",
    "mn",
    "mfrac",
    "msup",
    "msub",
    "msubsup",
    "munder",
    "mover",
    "munderover",
    "mtable",
    "mtr",
    "mtd",
    "mtext",
    "mspace",
  ],
};

export const Markdown = ({
  content = "",
  className = "text-base leading-relaxed text-base-content",
}) => (
  <div className={className}>
    <ReactMarkdown
      remarkPlugins={[remarkGfm, [remarkMath, { singleDollarTextMath: true }]]}
      rehypePlugins={[
        rehypeRaw,
        [rehypeKatex, { output: "mathml" }],
        [rehypeSanitize, mathSafeSchema],
      ]}
      components={{
        code: CodeBlock,
        img: Img,
        a: Link,
        details: ThinkBlock,
      }}
    >
      {content}
    </ReactMarkdown>
  </div>
);


export function StaggeredMarkdown({ content }: { content: string }) {
  const words = content.split(/(\s+)/); // 保留空白，避免标点粘连
  return (
      <span className="lm-fade-container">
      {words.map((w, i) => (
          <span
              key={i}
              className="lm-fade-word"
              style={{ animationDelay: `${i * 40}ms` }}
          >
          {w}
        </span>
      ))}
    </span>
  );
}