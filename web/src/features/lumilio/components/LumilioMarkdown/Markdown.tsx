// 只剩最小核心组件
import ReactMarkdown,{Components} from "react-markdown";
import remarkGfm from "remark-gfm";
import remarkMath from "remark-math";
import rehypeKatex from "rehype-katex";
import rehypeRaw from "rehype-raw";
import rehypeSanitize, { defaultSchema } from "rehype-sanitize";
import { CodeBlock, Img, Link, ThinkBlock } from "./MarkdownBlocks";
import { MarkdownToolBlock } from "./MarkdownBlocks/ToolBlock.tsx";


interface CustomComponents extends Partial<Components> {
  "lumilio-tool"?: React.ComponentType<any>;
  [key: string]: any; // 允许任意其他字符串键
}

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
    "lumilio-tool": ["id", "data-id", "className", "style"],
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
    "lumilio-tool",
  ],
};

const components: CustomComponents = {
  code: CodeBlock,
  img: Img,
  a: Link,
  details: ThinkBlock,
  "lumilio-tool": MarkdownToolBlock,
  p: ({node, ...props}) => <div {...props} />
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
      components={components}
    >
      {content}
    </ReactMarkdown>
  </div>
);
