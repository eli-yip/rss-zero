import { Card, CardBody } from "@heroui/react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import "github-markdown-css/github-markdown.css";

interface TopicProps {
  content: string;
}

export function Topic({ content }: TopicProps) {
  return (
    <div>
      <Card className="mb-4">
        <CardBody>
          <div className="markdown-body">
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
