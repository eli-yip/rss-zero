import type { Topic as TopicType } from "@/types/topic";

import {
  Button,
  ButtonGroup,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
} from "@heroui/react";
import moment from "moment";
import { FaArchive, FaCopy, FaSave, FaZhihu } from "react-icons/fa";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import "@/styles/github-markdown.css";

interface TopicProps {
  topic: TopicType;
}

function Topic({ topic }: TopicProps) {
  let archiveUrl = topic.archive_url;

  if (window.location.hostname === "mo.darkeli.com") {
    archiveUrl = archiveUrl.replace("rss-zero", "mo");
  }

  return (
    <div>
      <Card disableAnimation className="markdown-body mb-4">
        <CardHeader className="flex justify-between gap-1">
          <h3>{topic.title}</h3>
          <div className="flex flex-row justify-start sm:justify-end">
            <ButtonGroup>
              <PlatformLink platform="原文" url={topic.original_url} />
              <PlatformLink platform="存档" url={archiveUrl} />
              <Button
                isIconOnly
                size="sm"
                onPress={() =>
                  navigator.clipboard.writeText(
                    `# ${topic.title}\n\n${topic.body}`,
                  )
                }
              >
                <FaCopy />
              </Button>
              <Button
                isIconOnly
                size="sm"
                onPress={() => navigator.clipboard.writeText(archiveUrl)}
              >
                <FaSave />
              </Button>
            </ButtonGroup>
          </div>
        </CardHeader>
        <CardBody>
          <div>
            <Markdown remarkPlugins={[remarkGfm]}>{topic.body}</Markdown>
          </div>
        </CardBody>
        <CardFooter className="flex flex-col justify-between gap-2 font-bold sm:flex-row">
          <span>{topic.author.nickname}</span>
          <span>{moment(topic.created_at).format("YYYY年M月D日 HH:mm")}</span>
        </CardFooter>
      </Card>
    </div>
  );
}

interface PlatformLinkProps {
  platform: string;
  url: string;
}

function PlatformLink({ platform, url }: PlatformLinkProps) {
  const icon = (platform: string) => {
    switch (platform) {
      case "原文":
        return <FaZhihu />;
      case "存档":
        return <FaArchive />;
    }
  };

  return (
    <Button isIconOnly size="sm" onPress={() => window.open(url, "_blank")}>
      {icon(platform)}
    </Button>
  );
}

interface TopicsProps {
  topics: TopicType[];
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
