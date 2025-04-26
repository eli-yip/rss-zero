import {
  Button,
  DatePicker,
  type DateValue,
  Listbox,
  ListboxItem,
  ListboxSection,
  Popover,
  PopoverContent,
  PopoverTrigger,
  Switch,
} from "@heroui/react";
import { parseDate } from "@internationalized/date";
import { useEffect, useMemo, useState } from "react";
import { FaFilter, FaTags } from "react-icons/fa";

import { TimeBy } from "@/api/client";
import { DateClearButtons } from "@/components/DateClearButtons";
import { TagInputForm } from "@/components/topic/TagInput";
import { SORT_OPTIONS } from "@/constants/archive";
import { useAllTags } from "@/hooks/useAllTags";

interface BookmarkFiltersProps {
  tagInclude: string[] | null;
  tagExclude: string[] | null;
  noTag: boolean;
  startDate: string;
  endDate: string;
  dateBy: TimeBy;
  order: number;
  orderBy: TimeBy;
  updateSearchParams: (updates: Record<string, string | null>) => void;
}

export function BookmarkFilters({
  tagInclude,
  tagExclude,
  noTag,
  startDate,
  endDate,
  dateBy,
  order,
  orderBy,
  updateSearchParams,
}: BookmarkFiltersProps) {
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

  // 日期类型状态
  const [isCreateDate, setIsCreateDate] = useState(dateBy === TimeBy.Create);

  // 排序依据状态
  const [isOrderByCreate, setIsOrderByCreate] = useState(
    orderBy === TimeBy.Create,
  );

  // 获取所有标签
  const { tags, isLoading } = useAllTags();

  // 将标签数据转换为 TagInputForm 所需的格式
  const tagCountMap = Object.fromEntries(
    tags.map((tag) => [tag.name, tag.count]),
  );

  // 内部标签状态，用于表单显示
  const [showNoTag, setShowNoTag] = useState(noTag);

  // 同步排序开关状态
  useEffect(() => {
    setIsNewestFirst(order === SORT_OPTIONS.NEWEST_FIRST);
  }, [order]);

  // 同步日期类型状态
  useEffect(() => {
    setIsCreateDate(dateBy === TimeBy.Create);
  }, [dateBy]);

  // 同步排序依据状态
  useEffect(() => {
    setIsOrderByCreate(orderBy === TimeBy.Create);
  }, [orderBy]);

  // 同步无标签状态
  useEffect(() => {
    setShowNoTag(noTag);
  }, [noTag]);

  // 处理排序变更
  const handleSortChange = (isNewest: boolean) => {
    setIsNewestFirst(isNewest);
    updateSearchParams({
      order: isNewest
        ? String(SORT_OPTIONS.NEWEST_FIRST)
        : String(SORT_OPTIONS.OLDEST_FIRST),
    });
  };

  // 处理日期类型变更
  const handleDateByChange = (isCreate: boolean) => {
    setIsCreateDate(isCreate);
    updateSearchParams({
      dateBy: isCreate ? String(TimeBy.Create) : String(TimeBy.Update),
    });
  };

  // 处理排序依据变更
  const handleOrderByChange = (isCreate: boolean) => {
    setIsOrderByCreate(isCreate);
    updateSearchParams({
      orderBy: isCreate ? String(TimeBy.Create) : String(TimeBy.Update),
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

  // 处理包含标签变更
  const handleIncludeTagsChange = (tags: string[]) => {
    updateSearchParams({
      tagInclude: tags.length > 0 ? tags.join(",") : null,
    });
  };

  // 处理取消编辑包含标签
  const handleCancelIncludeTagsEdit = () => {
    // 取消编辑，不执行任何操作，保持原状态
  };

  // 处理排除标签变更
  const handleExcludeTagsChange = (tags: string[]) => {
    updateSearchParams({
      tagExclude: tags.length > 0 ? tags.join(",") : null,
    });
  };

  // 处理取消编辑排除标签
  const handleCancelExcludeTagsEdit = () => {
    // 取消编辑，不执行任何操作，保持原状态
  };

  // 处理无标签选项变更
  const handleNoTagChange = (value: boolean) => {
    setShowNoTag(value);
    updateSearchParams({
      noTag: value ? "true" : "false",
    });
  };

  return (
    <div className="mx-auto flex w-full max-w-xs flex-col pb-4">
      <div className="mx-auto flex w-full gap-4">
        {/* 标签筛选按钮 */}
        <Popover>
          <PopoverTrigger className="w-1/2">
            <Button startContent={<FaTags />}>标签筛选</Button>
          </PopoverTrigger>
          <PopoverContent className="w-80 flex-col gap-4 p-4">
            <div className="flex flex-col gap-4">
              <h3 className="font-bold">包含标签</h3>
              {isLoading ? (
                <div>加载标签中...</div>
              ) : (
                <TagInputForm
                  tagCountMap={tagCountMap}
                  value={tagInclude || []}
                  onChange={handleIncludeTagsChange}
                  onCancel={handleCancelIncludeTagsEdit}
                  placeholder="输入需要包含的标签"
                />
              )}

              <h3 className="mt-2 font-bold">排除标签</h3>
              {isLoading ? (
                <div>加载标签中...</div>
              ) : (
                <TagInputForm
                  tagCountMap={tagCountMap}
                  value={tagExclude || []}
                  onChange={handleExcludeTagsChange}
                  onCancel={handleCancelExcludeTagsEdit}
                  placeholder="输入需要排除的标签"
                />
              )}

              <Switch
                isSelected={showNoTag}
                onValueChange={handleNoTagChange}
                className="mt-2 w-full"
              >
                只显示没有标签的内容
              </Switch>
            </div>
          </PopoverContent>
        </Popover>

        {/* 更多选项弹出框 */}
        <Popover>
          <PopoverTrigger className="w-1/2">
            <Button startContent={<FaFilter />}>更多筛选</Button>
          </PopoverTrigger>
          <PopoverContent className="w-64 flex-col gap-4 px-4 py-4">
            {/* 日期选择器 */}
            <Listbox
              aria-label="筛选"
              itemClasses={{
                base: "data-[focus]:bg-transparent data-[hover]:bg-transparent",
              }}
            >
              <ListboxSection showDivider title={"按照日期筛选"}>
                <ListboxItem textValue="按创建时间">
                  <Switch
                    isSelected={isCreateDate}
                    onValueChange={handleDateByChange}
                    className="w-full"
                  >
                    {isCreateDate ? "按创建时间筛选" : "按更新时间筛选"}
                  </Switch>
                </ListboxItem>
                <ListboxItem textValue="开始时间">
                  <DatePicker
                    showMonthAndYearPickers
                    label="开始时间"
                    value={localStartDate}
                    onChange={handleStartDateChange}
                  />
                </ListboxItem>
                <ListboxItem textValue="截止时间">
                  <DatePicker
                    showMonthAndYearPickers
                    label="截止时间"
                    value={localEndDate}
                    onChange={handleEndDateChange}
                  />
                </ListboxItem>
                <ListboxItem textValue="清除日期">
                  {/* 日期清除按钮组件 */}
                  <DateClearButtons
                    startDate={localStartDate}
                    endDate={localEndDate}
                    onClearStartDate={handleClearStartDate}
                    onClearEndDate={handleClearEndDate}
                  />
                </ListboxItem>
              </ListboxSection>

              <ListboxSection showDivider title={"排序设置"}>
                <ListboxItem textValue="按创建时间排序">
                  <Switch
                    isSelected={isOrderByCreate}
                    onValueChange={handleOrderByChange}
                    className="w-full"
                  >
                    {isOrderByCreate ? "按创建时间排序" : "按更新时间排序"}
                  </Switch>
                </ListboxItem>
                <ListboxItem textValue="排序方式">
                  <Switch
                    isSelected={isNewestFirst}
                    onValueChange={handleSortChange}
                    className="w-full"
                  >
                    {isNewestFirst ? "从新到旧" : "从旧到新"}
                  </Switch>
                </ListboxItem>
              </ListboxSection>
            </Listbox>
          </PopoverContent>
        </Popover>
      </div>
    </div>
  );
}
