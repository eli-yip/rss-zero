import { Button, Input, Pagination } from "@heroui/react";
import { useCallback, useState } from "react";
import { FaEye, FaEyeSlash } from "react-icons/fa";

interface ArchivePaginationProps {
  page: number;
  total: number;
  onChange: (page: number) => void;
}

/**
 * 归档分页组件，支持移动端和桌面端显示
 */
export function ArchivePagination({
  page,
  total,
  onChange,
}: ArchivePaginationProps) {
  const [mobilePaginationVisible, setMobilePaginationVisible] = useState(false);

  const toggleMobilePagination = useCallback(() => {
    setMobilePaginationVisible((prev) => !prev);
  }, []);

  return (
    <div>
      {/* 桌面端分页，始终显示 */}
      <div className="hidden sm:block">
        <PaginationWithJumper page={page} total={total} onChange={onChange} />
      </div>

      {/* 移动端分页，可切换显示 */}
      <div className="sm:hidden">
        <Button
          isIconOnly
          className="fixed bottom-4 left-4 z-50 rounded-full"
          onPress={toggleMobilePagination}
        >
          {mobilePaginationVisible ? <FaEye /> : <FaEyeSlash />}
        </Button>

        {mobilePaginationVisible && (
          <PaginationWithJumper page={page} total={total} onChange={onChange} />
        )}
      </div>
    </div>
  );
}

/**
 * 带跳转功能的分页组件
 */
function PaginationWithJumper({
  page,
  total,
  onChange,
}: ArchivePaginationProps) {
  const [inputPage, setInputPage] = useState<string>("");

  // 处理页面跳转
  const handleJump = useCallback(() => {
    const pageNum = Number.parseInt(inputPage);

    if (!Number.isNaN(pageNum) && pageNum > 0 && pageNum <= total) {
      onChange(pageNum);
      setInputPage("");
    }
  }, [inputPage, total, onChange]);

  // 处理回车键跳转
  const handleKeyPress = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === "Enter") {
        handleJump();
      }
    },
    [handleJump],
  );

  // 处理输入变更
  const handleInputChange = useCallback(
    (e: React.ChangeEvent<HTMLInputElement>) => {
      setInputPage(e.target.value);
    },
    [],
  );

  return (
    <div className="fixed right-0 bottom-4 left-0 z-40 flex flex-col items-center justify-center gap-4">
      {/* 分页控件 */}
      <Pagination
        isCompact
        showControls
        page={page}
        total={total}
        onChange={onChange}
      />

      {/* 页面跳转输入框 */}
      <div className="flex items-center gap-2">
        <Input
          className="w-10"
          fullWidth={false}
          placeholder=""
          value={inputPage}
          onChange={handleInputChange}
          onKeyDown={handleKeyPress}
        />
        <Button isIconOnly onPress={handleJump}>
          Go
        </Button>
      </div>
    </div>
  );
}
