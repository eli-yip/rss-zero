import { useSearchParams, useNavigate } from "react-router-dom";
import { Button, Input, Pagination } from "@heroui/react";

import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";
import { apiUrl } from "@/config/config";
import { useEffect, useState } from "react";
import { Topic } from "@/types/topic";
import { Topics } from "@/components/topic";
import { ScrollToTop } from "@/components/scroll-to-top";
import { scrollToTop } from "@/utils/window";

interface Request {
  /**
   * 作者url token，例如：canglimo
   */
  author: string;
  /**
   * 数量
   */
  count: number;
  /**
   * 页码
   */
  page: number;
  /**
   * 平台
   */
  platform: string;
  /**
   * 类型
   */
  type: string;
}
interface Response {
  /**
   * 总数量
   */
  count: number;
  /**
   * 分页信息
   */
  paging: Paging;
  topics: Topic[];
  [property: string]: any;
}

/**
 * 分页信息
 */
interface Paging {
  /**
   * 当前页码
   */
  current: number;
  /**
   * 页数
   */
  total: number;
}

export default function ArchivePage() {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [searchParams] = useSearchParams();
  const page = searchParams.get("page") || "1";
  const navigate = useNavigate();

  const getTopics = async (page: number) => {
    const requestBody: Request = {
      platform: "zhihu",
      type: "answer",
      count: 10,
      page: page,
      author: "canglimo",
    };

    const response = await fetch(`${apiUrl}/archive`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
      },
      body: JSON.stringify(requestBody),
    });

    const data: Response = await response.json();
    setTopics(data.topics);
    setTotal(data.count);
  };

  const handlePageChange = (page: number) => {
    navigate(`/archive?page=${page}`);
    scrollToTop();
  };

  useEffect(() => {
    getTopics(parseInt(page));
  }, [page]);

  return (
    <DefaultLayout>
      <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
        <div className="inline-block max-w-lg justify-center text-center">
          <h1 className={title()}>历史文章</h1>
        </div>
      </section>
      <Topics topics={topics} />
      <ScrollToTop />
      <PaginationWrapper
        page={parseInt(page)}
        total={total}
        onChange={handlePageChange}
      />
    </DefaultLayout>
  );
}

interface PaginationWrapperProps {
  page: number;
  total: number;
  onChange: (page: number) => void;
}

function PaginationWrapper({ page, total, onChange }: PaginationWrapperProps) {
  const [inputPage, setInputPage] = useState<string>("");

  const handleJump = () => {
    const pageNum = parseInt(inputPage);
    if (!isNaN(pageNum) && pageNum > 0 && pageNum <= total) {
      onChange(pageNum);
      setInputPage("");
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleJump();
    }
  };

  return (
    <div className="fixed bottom-8 left-0 right-0 z-40 flex flex-col items-center justify-center gap-2">
      <Pagination showControls page={page} total={total} onChange={onChange} />
      <div className="flex items-center gap-2">
        <Input
          className="w-20"
          value={inputPage}
          onChange={(e) => setInputPage(e.target.value)}
          placeholder="页码"
          onKeyDown={handleKeyPress}
        />
        <Button onPress={handleJump}>跳转</Button>
      </div>
    </div>
  );
}
