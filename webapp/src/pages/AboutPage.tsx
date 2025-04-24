import { Card, CardBody } from "@heroui/react";
import Markdown from "react-markdown";
import remarkGfm from "remark-gfm";

import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";

import "@/styles/github-markdown.css";

const roadMap = `\
## 一份想写但是偷懒没写的路线图...

- [ ] 完善知乎的搜索功能，添加多重筛选（作者、内容类型、时间等）
- [ ] 支持通过标签对内容进行收藏和整理，支持对内容添加备注和笔记
  - [x] 实现标签功能，支持在「我的书签」页面对内容进行筛选
  - [ ] 实现笔记和备注功能
- [ ] 支持手动标记内容的阅读次数，表示内容的权重
- [ ] 实现墨大为了解决灾难化叙事而推荐的「双面卡片」功能

本站起初只是我自用的内容存档，甚至没有实现前端的打算，上述功能也仅仅是我的想法，并不一定会实现。
`;

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
          <CardBody className="markdown-body">
            <div className="mx-4">
              <Markdown remarkPlugins={[remarkGfm]}>{roadMap}</Markdown>
            </div>
          </CardBody>
        </Card>
      </div>
    </DefaultLayout>
  );
}
