import { DatePicker, DateValue, Select, SelectItem } from "@heroui/react";
import { parseDate } from "@internationalized/date";
import { useSearchParams } from "react-router-dom";

import { ContentType } from "@/api/archive";
import { Archive } from "@/components/archive";
import { title } from "@/components/primitives";
import { useArchiveTopics } from "@/hooks/use-archive-topics";
import DefaultLayout from "@/layouts/default";
import { scrollToTop } from "@/utils/window";

export default function ArchivePage() {
  const [searchParams, setSearchParams] = useSearchParams();
  const pageStr = searchParams.get("page") || "1";
  const page = parseInt(pageStr);
  const startDate = searchParams.get("startDate") || "";
  const endDate = searchParams.get("endDate") || "";
  const isValidContentType = (value: string | null): value is ContentType => {
    return (
      value !== null &&
      Object.values(ContentType).includes(value as ContentType)
    );
  };
  const contentType = isValidContentType(searchParams.get("contentType"))
    ? (searchParams.get("contentType") as ContentType)
    : ContentType.Answer;
  const { topics, total, firstFetchDone, loading } = useArchiveTopics(
    page,
    startDate,
    endDate,
    contentType,
  );

  const updateDateParam = (param: string, value: DateValue | null) => {
    const dateValue = value ? value.toString() : "";
    const params = new URLSearchParams(searchParams);

    params.set("page", "1");

    if (dateValue) params.set(param, dateValue);
    else params.delete(param);

    setSearchParams(params);
    scrollToTop();
  };

  const handleContentTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const params = new URLSearchParams(searchParams);

    params.set("contentType", e.target.value);
    setSearchParams(params);
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

      <div className="mx-auto flex w-full max-w-xs flex-col pb-4">
        <div className="mx-auto my-4 flex w-full flex-col gap-4 sm:flex-row">
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
        <Select
          defaultSelectedKeys={[contentType]}
          value={contentType}
          onChange={handleContentTypeChange}
        >
          <SelectItem key={ContentType.Answer}>回答</SelectItem>
          <SelectItem key={ContentType.Pin}>想法</SelectItem>
        </Select>
      </div>

      <Archive
        firstFetchDone={firstFetchDone}
        loading={loading}
        page={page}
        setSearchParams={setSearchParams}
        topics={topics}
        total={total}
      />
    </DefaultLayout>
  );
}
