import { useCallback } from "react";

import { Topics } from "@/components/topic";
import { Topic } from "@/types/topic";
import { scrollToTop } from "@/utils/window";

import PaginationWrapper from "./pagination";
import { ScrollToTop } from "./scroll-to-top";

interface ArchiveProps {
  loading: boolean;
  topics: Topic[];
  page: number;
  total: number;
  firstFetchDone: boolean;
  setSearchParams: (params: URLSearchParams) => void;
}

export function Archive({
  loading,
  topics,
  page,
  total,
  firstFetchDone,
  setSearchParams,
}: ArchiveProps) {
  const handlePageChange = useCallback(
    (page: number) => {
      const currentSearchParams = new URLSearchParams(window.location.search);

      currentSearchParams.set("page", page.toString());
      setSearchParams(currentSearchParams);
      scrollToTop();
    },
    [setSearchParams],
  );

  return (
    <div className="mb-16">
      {!loading && topics && <Topics topics={topics} />}
      <ScrollToTop />
      {/**
       * Note: Why we need to check firstFetchDone here?
       * HeroUI Pagination componet will not highlight current page in case that
       * the total page is 0, no matter whether the total page is updated or not.
       * So we need to make sure that the total page is updated before rendering.
       * Besides, we want to show the pagination component when changing pages.
       * So only when browser is fetching data for the first time,
       * we don't show the pagination component.
       */}
      {firstFetchDone && (
        <PaginationWrapper
          page={page}
          total={total}
          onChange={handlePageChange}
        />
      )}
    </div>
  );
}
