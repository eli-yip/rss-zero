import { Markdown } from "@/components/topic/Markdown";
import { Card, CardBody } from "@heroui/react";

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

## 更新日志

### v1.13

#### v1.13.8

发布时间：TBD

- **支持清除已经设置的日期范围**
- **优化了书签高级筛选的样式**
- **优化编辑备注的体验**：
  - 点击「编辑备注」按钮时，自动聚焦到输入框
  - 使用 Enter 键保存备注，使用 Esc 键取消编辑

#### v1.13.7

发布时间：*2025.4.25*

- **添加了备注功能**
- **更改标签时自动去重**
- 在「随便看看」底部添加了刷新按钮
- 修复了 Markdown 列表样式问题

#### v1.13.6

发布时间：*2025.4.24*

- 修复了内容展示的 Markdown 样式

#### v1.13.5

发布时间：*2025.4.24*

- 添加了「FanFanFan」作为内容作者

#### v1.13.4

发布时间：*2025.4.24*

- 美化了标签选取列表的样式

#### v1.13.3

发布时间：*2025.4.24*

- 修复了标签选取列表的样式
- 优化了标签选取列表的性能

#### v1.13.2

发布时间：*2025.4.24*

- **修复了标签数量过多时的样式问题**

#### v1.13.1

发布时间：*2025.4.24*

- 展示昵称而不是用户名
- 当用户为测试用户时，禁用书签按钮

#### v1.13.0

发布时间：*2025.4.24*

- **添加了书签和标签功能**
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
          <CardBody>
            <Markdown content={roadMap} />
          </CardBody>
        </Card>
      </div>
    </DefaultLayout>
  );
}
