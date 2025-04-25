import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";

import "@/styles/github-markdown.css";

interface MarkdownProps {
  content: string;
}

export function Markdown({ content }: MarkdownProps) {
  return (
    <div className="markdown-body">
      <ReactMarkdown remarkPlugins={[remarkGfm]}>{content}</ReactMarkdown>
    </div>
  );
}
