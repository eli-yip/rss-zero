import { useSearchParams, useNavigate } from "react-router-dom";
import { DatePicker, DateValue } from "@heroui/react";
import { parseDate } from "@internationalized/date";

import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";
import { Topics } from "@/components/topic";
import { ScrollToTop } from "@/components/scroll-to-top";
import { scrollToTop } from "@/utils/window";
import PaginationWrapper from "@/components/pagination";
import { useArchiveTopics } from "@/hooks/use-archive-topics";

export default function ArchivePage() {
  const navigate = useNavigate();

  const [searchParams] = useSearchParams();
  const pageStr = searchParams.get("page") || "1";
  const page = parseInt(pageStr);
  const startDate = searchParams.get("startDate") || "";
  const endDate = searchParams.get("endDate") || "";
  const { topics, total, firstFetchDone, loading } = useArchiveTopics(
    page,
    startDate,
    endDate,
  );

  const handlePageChange = (page: number) => {
    const params = new URLSearchParams(searchParams);

    params.set("page", page.toString());
    navigate(`/archive?${params.toString()}`);
    scrollToTop();
  };

  const updateDateParam = (param: string, value: DateValue | null) => {
    const dateValue = value ? value.toString() : "";
    const params = new URLSearchParams(searchParams);

    params.set("page", "1");

    if (dateValue) params.set(param, dateValue);
    else params.delete(param);

    navigate(`/archive?${params.toString()}`);
    scrollToTop();
  };

  const handleStartDateChange = (value: DateValue | null) => {
    updateDateParam("startDate", value);
  };

  const handleEndDateChange = (value: DateValue | null) => {
    updateDateParam("endDate", value);
  };

  return (
    <DefaultLayout>
      <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
        <div className="inline-block max-w-lg justify-center text-center">
          <h1 className={title()}>历史文章</h1>
        </div>
      </section>

      <div className="flex flex-col sm:flex-row w-full max-w-xs mx-auto my-4 gap-4">
        <DatePicker
          showMonthAndYearPickers
          label="开始时间"
          value={startDate ? parseDate(startDate) : null}
          onChange={handleStartDateChange}
        />
        <DatePicker
          showMonthAndYearPickers
          label="截止时间"
          value={endDate ? parseDate(endDate) : null}
          onChange={handleEndDateChange}
        />
      </div>

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
    </DefaultLayout>
  );
}
