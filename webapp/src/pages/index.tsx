import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";
import { Card, CardBody, CardHeader } from "@heroui/react";

export default function IndexPage() {
  return (
    <DefaultLayout>
      <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
        <div className="inline-block max-w-lg justify-center text-center">
          <span className={title()}>墨家&nbsp;</span>
          <span className={title({ color: "violet" })}>第一基地&nbsp;</span>
        </div>
      </section>

      <div className="mx-auto max-w-3xl w-full">
        <Card>
          <CardHeader className="justify-center">
            <p className="text-xl font-bold">使用说明</p>
          </CardHeader>
          <CardBody>
            <div className="mx-6">
              <ul className="list-disc leading-loose text-pretty">
                <li>随便看看中有随机刷新的墨大回答</li>
                <li>历史文章中可以根据日期、作者、内容类型筛选和浏览</li>
                <li>
                  文章数据中展示了墨大过去一年的创作热力图，点击具体的日期可以显示那天的内容
                </li>
                <li>直播回放中汇总了从 2024 年 10 月开始的所有直播回放链接</li>
              </ul>
            </div>
          </CardBody>
        </Card>
      </div>
    </DefaultLayout>
  );
}
