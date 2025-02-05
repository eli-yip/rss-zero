import { useSearchParams, useNavigate } from "react-router-dom";
import { useEffect, useState } from "react";

import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";
import { apiUrl } from "@/config/config";
import { Topic } from "@/types/topic";
import { Topics } from "@/components/topic";
import { ScrollToTop } from "@/components/scroll-to-top";
import { scrollToTop } from "@/utils/window";
import PaginationWrapper from "@/components/pagination";

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
  const [firstFetch, setFirstFetch] = useState<boolean>(false);
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
    setTotal(data.paging.total);
    setFirstFetch(true);
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
      {firstFetch && (
        <PaginationWrapper
          page={parseInt(page)}
          total={total}
          onChange={handlePageChange}
        />
      )}
    </DefaultLayout>
  );
}
