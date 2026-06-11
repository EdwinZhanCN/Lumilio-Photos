// 只剩最小核心组件
import ReactMarkdown, { Components } from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import { CodeBlock, Img, Link } from "./MarkdownBlocks";

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

const components: Partial<Components> = {
  code: CodeBlock,
  img: Img,
  a: Link,
  p: (props) => <div {...props} />,
};

export const Markdown = ({
  content = "",
  className = "text-base leading-relaxed",
}) => (
  <div className={className}>
    <ReactMarkdown
      remarkPlugins={[remarkGfm, [remarkMath, { singleDollarTextMath: true }]]}
      rehypePlugins={[
        rehypeRaw,
        [rehypeKatex, { output: "mathml" }],
        [rehypeSanitize, mathSafeSchema],
      ]}
      components={components}
    >
      {content}
    </ReactMarkdown>
  </div>
);
