import { addToast } from "@heroui/react";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { TimeBy } from "@/api/client";
import { Archive } from "@/components/archive/Archive";
import { BookmarkFilters } from "@/components/bookmark/BookmarkFilter";
import { title } from "@/components/primitives";
import { SORT_OPTIONS, isValidOrder } from "@/constants/archive";
import { useBookmarkList } from "@/hooks/useBookmarkList";
import DefaultLayout from "@/layouts/default";
import type { Topic } from "@/types/Topic";
import { scrollToTop } from "@/utils/window";

export default function BookmarkPage() {
  // 搜索参数状态
  const [searchParams, setSearchParams] = useSearchParams();

  // 从 URL 获取参数
  const orderStr = searchParams.get("order") || "0";
  const pageStr = searchParams.get("page") || "1";
  const startDate = searchParams.get("startDate") || "";
  const endDate = searchParams.get("endDate") || "";
  const tagIncludeStr = searchParams.get("tagInclude") || "";
  const tagExcludeStr = searchParams.get("tagExclude") || "";
  const noTagStr = searchParams.get("noTag") || "false";
  const dateByStr = searchParams.get("dateBy") || "0";
  const orderByStr = searchParams.get("orderBy") || "0";

  // 参数解析
  const order = Number.parseInt(orderStr);
  const page = Number.parseInt(pageStr);
  const dateBy =
    Number.parseInt(dateByStr) === 0 ? TimeBy.Create : TimeBy.Update;
  const orderBy =
    Number.parseInt(orderByStr) === 0 ? TimeBy.Create : TimeBy.Update;
  const noTag = noTagStr === "true";

  // 解析标签数组
  const tagInclude = useMemo(
    () => (tagIncludeStr !== "" ? tagIncludeStr.split(",") : null),
    [tagIncludeStr],
  );

  const tagExclude = useMemo(
    () => (tagExcludeStr !== "" ? tagExcludeStr.split(",") : null),
    [tagExcludeStr],
  );

  // 在顶层组件中维护 topics 状态
  const [topics, setTopics] = useState<Topic[]>([]);

  // 获取书签数据
  const {
    topics: initialTopics,
    total,
    firstFetchDone,
    loading,
  } = useBookmarkList(
    page,
    tagInclude,
    tagExclude,
    noTag,
    startDate,
    endDate,
    dateBy,
    order,
    orderBy,
  );

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
  }, [orderStr, searchParams, setSearchParams]);

  // 更新搜索参数的处理函数
  const updateSearchParams = (updates: Record<string, string | null>) => {
    const newSearchParams = new URLSearchParams(searchParams);

    // 设置页码为 1，因为筛选条件变更后应该从第一页开始
    newSearchParams.set("page", "1");

    // 应用所有更新
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
          <h1 className={title()}>我的收藏</h1>
        </div>
      </section>

      {/* 筛选控件 */}
      <BookmarkFilters
        tagInclude={tagInclude}
        tagExclude={tagExclude}
        noTag={noTag}
        startDate={startDate}
        endDate={endDate}
        dateBy={dateBy}
        order={order}
        orderBy={orderBy}
        updateSearchParams={updateSearchParams}
      />

      {/* 文章列表 */}
      <Archive
        onlyBookmark={true}
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
