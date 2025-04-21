import {
  Button,
  DatePicker,
  type DateValue,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Select,
  SelectItem,
  Switch,
  addToast,
} from "@heroui/react";
import { parseDate } from "@internationalized/date";
import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";

import { ContentType } from "@/api/archive";
import { Archive } from "@/components/archive";
import { title } from "@/components/primitives";
import { useArchiveTopics } from "@/hooks/use-archive-topics";
import DefaultLayout from "@/layouts/default";
import { scrollToTop } from "@/utils/window";

const canglimoUrlToken: string = "canglimo";
const authors = [
  { key: canglimoUrlToken, name: "墨苍离" },
  { key: "zi-e-79-23", name: "别打扰我修道" },
  { key: "fu-lan-ke-yang", name: "弗兰克扬" },
];

const isValidAuthor = (value: string | null): boolean => {
  return value !== null && authors.some((author) => author.key === value);
};

export default function ArchivePage() {
  const [isSelected, setIsSelected] = useState(true);
  const [currentOrder, setCurrentOrder] = useState("从新到旧");
  const [searchParams, setSearchParams] = useSearchParams();
  const orderStr = searchParams.get("order") || "0";
  const order = Number.parseInt(orderStr);
  const pageStr = searchParams.get("page") || "1";
  const page = Number.parseInt(pageStr);
  const startDate = searchParams.get("startDate") || "";
  const endDate = searchParams.get("endDate") || "";

  const authorParam = searchParams.get("author");
  const author = isValidAuthor(authorParam)
    ? (authorParam as string)
    : canglimoUrlToken;

  // Use `useEffect` here to avoid infinite loop
  // never change state during rendering process
  useEffect(() => {
    if (authorParam !== null && !isValidAuthor(authorParam)) {
      const newSearchParams = new URLSearchParams(searchParams);
      newSearchParams.set("author", canglimoUrlToken);
      setSearchParams(newSearchParams);
      addToast({
        title: "当前作者还未添加，切换到墨苍离的内容",
        timeout: 3000,
      });
    }
  }, [authorParam, searchParams, setSearchParams]);

  useEffect(() => {
    if (order !== 0 && order !== 1) {
      const newSearchParams = new URLSearchParams(searchParams);
      newSearchParams.set("order", "0");
      setSearchParams(newSearchParams);
      addToast({
        title: "当前排序方式不支持，切换到默认排序",
        timeout: 3000,
      });
    }

    if (isSelected) {
      setCurrentOrder("从新到旧");
      const newSearchParam = new URLSearchParams(searchParams);
      newSearchParam.set("order", "0");
      setSearchParams(newSearchParam);
    } else {
      setCurrentOrder("从旧到新");
      const newSearchParam = new URLSearchParams(searchParams);
      newSearchParam.set("order", "1");
      setSearchParams(newSearchParam);
    }
  }, [order, isSelected, searchParams, setSearchParams]);

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
    author,
    order,
  );

  const updateDateParam = (param: string, value: DateValue | null) => {
    const dateValue = value ? value.toString() : "";

    const newSearchParams = new URLSearchParams(searchParams);
    newSearchParams.set("page", "1");

    if (dateValue) {
      newSearchParams.set(param, dateValue);
    } else {
      newSearchParams.delete(param);
    }

    setSearchParams(newSearchParams);
    scrollToTop();
  };

  const handleContentTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newSearchParams = new URLSearchParams(searchParams);
    newSearchParams.set("contentType", e.target.value);
    newSearchParams.set("page", "1");

    setSearchParams(newSearchParams);
    scrollToTop();
  };

  const handleStartDateChange = (value: DateValue | null) => {
    updateDateParam("startDate", value);
  };

  const handleEndDateChange = (value: DateValue | null) => {
    updateDateParam("endDate", value);
  };

  const handleAuthorChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    const newAuthor = e.target.value;
    const validAuthor = isValidAuthor(newAuthor) ? newAuthor : canglimoUrlToken;

    const newSearchParams = new URLSearchParams(searchParams);
    newSearchParams.set("author", validAuthor);
    newSearchParams.set("page", "1");

    setSearchParams(newSearchParams);
    scrollToTop();
  };

  return (
    <DefaultLayout>
      <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
        <div className="inline-block max-w-lg justify-center text-center">
          <h1 className={title()}>历史文章</h1>
        </div>
      </section>

      <div className="mx-auto flex w-full max-w-xs flex-col pb-4">
        <div className="mx-auto my-0 flex w-full gap-4">
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
        <div className="mx-auto my-4 flex w-full gap-4">
          <Select
            defaultSelectedKeys={[contentType]}
            value={contentType}
            onChange={handleContentTypeChange}
            className="w-1/2"
          >
            <SelectItem key={ContentType.Answer}>回答</SelectItem>
            <SelectItem key={ContentType.Pin}>想法</SelectItem>
          </Select>
          <Popover>
            <PopoverTrigger className="w-1/2">
              <Button>更多选项</Button>
            </PopoverTrigger>
            <PopoverContent className="w-48 flex-col gap-2 py-2">
              <Select
                defaultSelectedKeys={[author]}
                selectedKeys={[author]}
                onChange={handleAuthorChange}
              >
                {authors.map((item) => (
                  <SelectItem key={item.key}>{item.name}</SelectItem>
                ))}
              </Select>
              <Switch
                isSelected={isSelected}
                onValueChange={setIsSelected}
                className="w-full"
              >
                {currentOrder}
              </Switch>
            </PopoverContent>
          </Popover>
        </div>
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
