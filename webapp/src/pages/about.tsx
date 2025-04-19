import { Card, CardBody } from "@heroui/react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";

import "@/styles/github-markdown.css";

const roadMap = `\
一份想写但是偷懒没写的路线图...`;

export default function DocsPage() {
	return (
		<DefaultLayout>
			<section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
				<div className="inline-block max-w-lg justify-center text-center">
					<h1 className={title()}>关于本站</h1>
				</div>
			</section>

			<div className="mx-auto w-full max-w-3xl">
				<Card>
					<CardBody>
						<Markdown remarkPlugins={[remarkGfm]}>{roadMap}</Markdown>
					</CardBody>
				</Card>
			</div>
		</DefaultLayout>
	);
}
