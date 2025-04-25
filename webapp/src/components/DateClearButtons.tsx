import { Button, type DateValue } from "@heroui/react";

// 日期清除按钮组件
interface DateClearButtonsProps {
  startDate: DateValue | null;
  endDate: DateValue | null;
  onClearStartDate: () => void;
  onClearEndDate: () => void;
}

export function DateClearButtons({
  startDate,
  endDate,
  onClearStartDate,
  onClearEndDate,
}: DateClearButtonsProps) {
  if (!startDate && !endDate) return null;

  return (
    <div className="flex w-full gap-2">
      {startDate ? (
        <Button className="flex-1" onPress={onClearStartDate}>
          清除开始
        </Button>
      ) : null}
      {endDate ? (
        <Button className="flex-1" onPress={onClearEndDate}>
          清除结束
        </Button>
      ) : null}
    </div>
  );
}
