import { addToast } from "@heroui/react";
import { useCallback, useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { ContentType } from "@/api/client";
import { Archive } from "@/components/archive/Archive";
import { ArchiveFilters } from "@/components/archive/ArchiveFilters";
import { title } from "@/components/primitives";
import {
  AUTHORS,
  SORT_OPTIONS,
  isValidAuthor,
  isValidContentType,
  isValidOrder,
} from "@/constants/archive";
import { useArchiveTopics } from "@/hooks/use-archive-topics";
import DefaultLayout from "@/layouts/default";
import type { Topic } from "@/types/Topic";
import { scrollToTop } from "@/utils/window";

export default function ArchivePage() {
  // 搜索参数状态
  const [searchParams, setSearchParams] = useSearchParams();

  // 从 URL 获取参数
  const orderStr = searchParams.get("order") || "0";
  const pageStr = searchParams.get("page") || "1";
  const startDate = searchParams.get("startDate") || "";
  const endDate = searchParams.get("endDate") || "";
  const authorParam = searchParams.get("author");

  // 参数解析
  const order = Number.parseInt(orderStr);
  const page = Number.parseInt(pageStr);
  const author = isValidAuthor(authorParam)
    ? (authorParam as string)
    : AUTHORS[0].key;
  const contentType = isValidContentType(searchParams.get("contentType"))
    ? (searchParams.get("contentType") as ContentType)
    : ContentType.Answer;

  // 获取文章数据
  const {
    topics: initialTopics,
    total,
    firstFetchDone,
    loading,
  } = useArchiveTopics(page, startDate, endDate, contentType, author, order);

  // 在顶层组件中维护 topics 状态
  const [topics, setTopics] = useState<Topic[]>([]);

  // 当从 API 获取的初始数据变化时更新本地状态
  useEffect(() => {
    setTopics(initialTopics);
  }, [initialTopics]);

  // 处理收藏状态变化的回调函数
  const handleTopicsUpdate = useCallback((updatedTopics: Topic[]) => {
    setTopics(updatedTopics);
  }, []);

  // 参数验证和修正
  useEffect(() => {
    let paramsUpdated = false;
    const newSearchParams = new URLSearchParams(searchParams);

    // 检查作者参数
    if (authorParam !== null && !isValidAuthor(authorParam)) {
      newSearchParams.set("author", AUTHORS[0].key);
      paramsUpdated = true;
      addToast({
        title: "当前作者还未添加，切换到墨苍离的内容",
        timeout: 3000,
      });
    }

    // 检查排序参数
    if (!isValidOrder(orderStr)) {
      newSearchParams.set("order", String(SORT_OPTIONS.NEWEST_FIRST));
      paramsUpdated = true;
      addToast({
        title: "当前排序方式不支持，切换到默认排序",
        timeout: 3000,
      });
    }

    // 如果有参数更新，则应用更新
    if (paramsUpdated) {
      setSearchParams(newSearchParams);
    }
  }, [authorParam, orderStr, searchParams, setSearchParams]);

  // 更新搜索参数的处理函数
  const updateSearchParams = (updates: Record<string, string | null>) => {
    const newSearchParams = new URLSearchParams(searchParams);

    // 设置页码为 1，因为筛选条件变更后应该从第一页开始
    newSearchParams.set("page", "1"); // 应用所有更新
    for (const [key, value] of Object.entries(updates)) {
      if (value === null) {
        newSearchParams.delete(key);
      } else {
        newSearchParams.set(key, value);
      }
    }

    setSearchParams(newSearchParams);
    scrollToTop();
  };

  return (
    <DefaultLayout>
      {/* 页面标题 */}
      <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
        <div className="inline-block max-w-lg justify-center text-center">
          <h1 className={title()}>历史文章</h1>
        </div>
      </section>

      {/* 筛选控件 */}
      <ArchiveFilters
        contentType={contentType}
        author={author}
        startDate={startDate}
        endDate={endDate}
        order={order}
        updateSearchParams={updateSearchParams}
      />

      {/* 文章列表 */}
      <Archive
        firstFetchDone={firstFetchDone}
        loading={loading}
        page={page}
        setSearchParams={setSearchParams}
        topics={topics}
        total={total}
        onTopicsUpdate={handleTopicsUpdate}
      />
    </DefaultLayout>
  );
}
