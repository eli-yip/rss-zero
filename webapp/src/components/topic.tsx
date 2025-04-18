import type { Topic as TopicType } from "@/types/topic";

import {
	Button,
	ButtonGroup,
	Card,
	CardBody,
	CardFooter,
	CardHeader,
	Tooltip,
} from "@heroui/react";
import { DateTime } from "luxon";
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
			<Card disableAnimation className="markdown-body !mb-4">
				<CardHeader className="flex justify-between gap-1">
					<h3>{topic.title}</h3>
					<div className="flex flex-row justify-start sm:justify-end">
						<ButtonGroup>
							<PlatformLink platform="原文" url={topic.original_url} />
							<PlatformLink platform="存档" url={archiveUrl} />
							<Tooltip content="复制 Markdown 全文">
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
							</Tooltip>
							<Tooltip content="复制存档链接">
								<Button
									isIconOnly
									size="sm"
									onPress={() => navigator.clipboard.writeText(archiveUrl)}
								>
									<FaSave />
								</Button>
							</Tooltip>
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
					<span>
						{DateTime.fromISO(topic.created_at).toFormat(
							"yyyy 年 L 月 d 日 h:m",
						)}
					</span>
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

	const buildContent = (platform: string) => {
		return `打开${platform}链接`;
	};

	return (
		<Tooltip content={buildContent(platform)}>
			<Button isIconOnly size="sm" onPress={() => window.open(url, "_blank")}>
				{icon(platform)}
			</Button>
		</Tooltip>
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
