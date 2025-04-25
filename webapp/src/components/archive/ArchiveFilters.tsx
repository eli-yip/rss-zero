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
} from "@heroui/react";
import { parseDate } from "@internationalized/date";
import { useEffect, useMemo, useState } from "react";

import { ContentType } from "@/api/client";
import { DateClearButtons } from "@/components/DateClearButtons";
import { AUTHORS, SORT_OPTIONS } from "@/constants/archive";

interface ArchiveFiltersProps {
  contentType: ContentType;
  author: string;
  startDate: string;
  endDate: string;
  order: number;
  updateSearchParams: (updates: Record<string, string | null>) => void;
}

export function ArchiveFilters({
  contentType,
  author,
  startDate,
  endDate,
  order,
  updateSearchParams,
}: ArchiveFiltersProps) {
  const localStartDate = useMemo(() => {
    return startDate ? parseDate(startDate) : null;
  }, [startDate]);
  const localEndDate = useMemo(() => {
    return endDate ? parseDate(endDate) : null;
  }, [endDate]);

  // 排序状态
  const [isNewestFirst, setIsNewestFirst] = useState(
    order === SORT_OPTIONS.NEWEST_FIRST,
  );

  // 同步排序开关状态
  useEffect(() => {
    setIsNewestFirst(order === SORT_OPTIONS.NEWEST_FIRST);
  }, [order]);

  // 处理内容类型变更
  const handleContentTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    updateSearchParams({ contentType: e.target.value });
  };

  // 处理作者变更
  const handleAuthorChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
    updateSearchParams({ author: e.target.value });
  };

  // 处理排序变更
  const handleSortChange = (isNewest: boolean) => {
    setIsNewestFirst(isNewest);
    updateSearchParams({
      order: isNewest
        ? String(SORT_OPTIONS.NEWEST_FIRST)
        : String(SORT_OPTIONS.OLDEST_FIRST),
    });
  };

  // 处理开始日期变更
  const handleStartDateChange = (value: DateValue | null) => {
    updateSearchParams({
      startDate: value ? value.toString() : null,
    });
  };

  // 处理结束日期变更
  const handleEndDateChange = (value: DateValue | null) => {
    updateSearchParams({
      endDate: value ? value.toString() : null,
    });
  };

  // 清除开始日期
  const handleClearStartDate = () => {
    updateSearchParams({ startDate: null });
  };

  // 清除结束日期
  const handleClearEndDate = () => {
    updateSearchParams({ endDate: null });
  };

  return (
    <div className="mx-auto flex w-full max-w-xs flex-col pb-4">
      <div className="mx-auto flex w-full gap-4">
        {/* 内容类型选择器 */}
        <Select
          aria-label="内容类型"
          selectedKeys={[contentType]}
          onChange={handleContentTypeChange}
          className="w-1/2"
        >
          <SelectItem key={ContentType.Answer}>回答</SelectItem>
          <SelectItem key={ContentType.Pin}>想法</SelectItem>
        </Select>

        {/* 更多选项弹出框 */}
        <Popover>
          <PopoverTrigger className="w-1/2">
            <Button>更多选项</Button>
          </PopoverTrigger>
          <PopoverContent className="w-48 flex-col gap-2 py-2">
            {/* 作者选择器 */}
            <Select selectedKeys={[author]} onChange={handleAuthorChange}>
              {AUTHORS.map((item) => (
                <SelectItem key={item.key}>{item.name}</SelectItem>
              ))}
            </Select>

            {/* 日期选择器 */}
            <DatePicker
              showMonthAndYearPickers
              label="开始时间"
              value={localStartDate}
              onChange={handleStartDateChange}
            />
            <DatePicker
              showMonthAndYearPickers
              label="截止时间"
              value={localEndDate}
              onChange={handleEndDateChange}
            />

            {/* 日期清除按钮组件 */}
            <DateClearButtons
              startDate={localStartDate}
              endDate={localEndDate}
              onClearStartDate={handleClearStartDate}
              onClearEndDate={handleClearEndDate}
            />

            {/* 排序开关 */}
            <Switch
              isSelected={isNewestFirst}
              onValueChange={handleSortChange}
              className="w-full"
            >
              {isNewestFirst ? "从新到旧" : "从旧到新"}
            </Switch>
          </PopoverContent>
        </Popover>
      </div>
    </div>
  );
}
