import { Card, CardBody } from "@heroui/react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import "@/styles/github-markdown.css";

interface TopicProps {
  content: string;
}

export function Topic({ content }: TopicProps) {
  return (
    <div>
      <Card disableAnimation className="mb-4 markdown-body">
        <CardBody>
          <div>
            <Markdown remarkPlugins={[remarkGfm]}>{content}</Markdown>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}

interface TopicsProps {
  contents: string[];
}

export function Topics({ contents }: TopicsProps) {
  return (
    <div>
      {contents.map((content, index) => (
        <Topic key={index} content={content} />
      ))}
    </div>
  );
}
