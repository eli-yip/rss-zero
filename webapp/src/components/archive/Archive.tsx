import { useCallback } from "react";

import { ArchivePagination } from "@/components/archive/ArchivePagination";
import { ScrollToTop } from "@/components/scroll-to-top";
import { Topics } from "@/components/topic/Topics";
import type { Topic } from "@/types/Topic";
import { scrollToTop } from "@/utils/window";

interface ArchiveProps {
  loading: boolean;
  topics: Topic[];
  page: number;
  total: number;
  firstFetchDone: boolean;
  onlyBookmark?: boolean;
  setSearchParams: (params: URLSearchParams) => void;
  onTopicsUpdate: (updatedTopics: Topic[]) => void;
}

/**
 * 文章归档组件，显示文章列表和分页
 */
export function Archive({
  loading,
  topics,
  page,
  total,
  firstFetchDone,
  onlyBookmark = false,
  setSearchParams,
  onTopicsUpdate,
}: ArchiveProps) {
  /**
   * 处理页面变更，更新 URL 参数并滚动到页面顶部
   */
  const handlePageChange = useCallback(
    (newPage: number) => {
      const currentSearchParams = new URLSearchParams(window.location.search);
      currentSearchParams.set("page", newPage.toString());
      setSearchParams(currentSearchParams);
      scrollToTop();
    },
    [setSearchParams],
  );

  return (
    <div className="mb-16">
      {/* 仅在加载完成且有数据时显示文章列表 */}
      {!loading && topics && topics.length > 0 ? (
        <Topics
          topics={topics}
          onTopicsChange={onTopicsUpdate}
          onlyBookmark={onlyBookmark}
        />
      ) : (
        !loading &&
        firstFetchDone && (
          <div className="mx-auto mt-8 text-center text-gray-500">暂无数据</div>
        )
      )}

      {/* 回到顶部按钮 */}
      <ScrollToTop />

      {/*
       * 仅在首次数据加载完成后显示分页组件
       * 这确保了分页组件在页面总数更新后才显示
       */}
      {firstFetchDone && total > 0 && (
        <ArchivePagination
          page={page}
          total={total}
          onChange={handlePageChange}
        />
      )}
    </div>
  );
}
