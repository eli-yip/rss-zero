import type { Topic } from "@/types/topic";

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
import { FaArchive } from "react-icons/fa";
import { AiOutlineZhihu } from "react-icons/ai";

import "@/styles/github-markdown.css";

interface TopicProps {
  topic: Topic;
}

export function Topic({ topic }: TopicProps) {
  let archiveUrl = topic.archive_url;

  if (window.location.hostname == "mo.darkeli.com") {
    archiveUrl = archiveUrl.replace("rss-zero", "mo");
  }

  return (
    <div>
      <Card disableAnimation className="markdown-body mb-4">
        <CardHeader className="flex justify-between gap-1">
          <h3>{topic.title}</h3>
          <div className="flex flex-row justify-start sm:justify-end gap-1">
            <PlatformLink platform="原文" url={topic.original_url} />
            <PlatformLink platform="存档" url={archiveUrl} />
          </div>
        </CardHeader>
        <CardBody>
          <div>
            <Markdown remarkPlugins={[remarkGfm]}>{topic.body}</Markdown>
          </div>
        </CardBody>
        <CardFooter className="flex flex-col justify-between gap-2 sm:flex-row">
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
  const icon = (platform: string) => {
    switch (platform) {
      case "原文":
        return <AiOutlineZhihu />;
      case "存档":
        return <FaArchive />;
    }
  };

  return (
    <Link href={url} target="_blank">
      <Button isIconOnly size="sm">
        {icon(platform)}
      </Button>
    </Link>
  );
}

interface TopicsProps {
  topics: Topic[];
}

export function Topics({ topics }: TopicsProps) {
  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col">
      {topics.map((topic) => (
        <Topic key={topic.id} topic={topic} />
      ))}
    </div>
  );
}
