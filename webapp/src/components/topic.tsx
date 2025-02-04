import {
  Button,
  Card,
  CardHeader,
  CardBody,
  CardFooter,
  Link,
} from "@heroui/react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";
import "@/styles/github-markdown.css";
import type { Topic } from "@/types/topic";

interface TopicProps {
  topic: Topic;
}

export function Topic({ topic }: TopicProps) {
  return (
    <div>
      <Card disableAnimation className="markdown-body mb-4">
        <CardHeader className="flex flex-row justify-between">
          <h3>{topic.title}</h3>
          <div className="flex flex-row justify-end">
            <PlatformLink platform="原文" url={topic.original_url} />
            <PlatformLink platform="存档" url={topic.archive_url} />
          </div>
        </CardHeader>
        <CardBody>
          <div>
            <Markdown remarkPlugins={[remarkGfm]}>{topic.body}</Markdown>
          </div>
        </CardBody>
        <CardFooter className="flex flex-row justify-between">
          <span>作者：{topic.author.nickname}</span>
          <span>创建时间：{new Date(topic.created_at).toLocaleString()}</span>
        </CardFooter>
      </Card>
    </div>
  );
}

interface PlatformLinkProps {
  platform: string;
  url: string;
}

export function PlatformLink({ platform, url }: PlatformLinkProps) {
  return (
    <Link href={url} target="_blank" className="m-2">
      <Button>{platform}</Button>
    </Link>
  );
}

interface TopicsProps {
  topics: Topic[];
}

export function Topics({ topics }: TopicsProps) {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col">
      {topics.map((topic, index) => (
        <Topic key={index} topic={topic} />
      ))}
    </div>
  );
}
