import { Button, Input, Pagination } from "@heroui/react";
import { useState } from "react";
import { FaEye, FaEyeSlash } from "react-icons/fa";

interface PaginationWrapperProps {
  page: number;
  total: number;
  onChange: (page: number) => void;
}

export default function PaginationWrapper({
  page,
  total,
  onChange,
}: PaginationWrapperProps) {
  const [mobilePaginationVisible, setMobilePaginationVisible] = useState(false);

  return (
    <div>
      <div className="hidden sm:block">
        <PaginationWithJumper page={page} total={total} onChange={onChange} />
      </div>

      <div className="sm:hidden">
        <Button
          isIconOnly
          className="fixed bottom-4 left-4 z-50 rounded-full"
          onPress={() => setMobilePaginationVisible(!mobilePaginationVisible)}
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

function PaginationWithJumper({
  page,
  total,
  onChange,
}: PaginationWrapperProps) {
  const [inputPage, setInputPage] = useState<string>("");
  const handleJump = () => {
    const pageNum = Number.parseInt(inputPage);

    if (!Number.isNaN(pageNum) && pageNum > 0 && pageNum <= total) {
      onChange(pageNum);
      setInputPage("");
    }
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter") {
      handleJump();
    }
  };

  return (
    <div className="fixed right-0 bottom-4 left-0 z-40 flex flex-col items-center justify-center gap-4">
      <Pagination
        isCompact
        showControls
        page={page}
        total={total}
        onChange={onChange}
      />
      <div className="flex items-center gap-2">
        <Input
          className="w-10"
          fullWidth={false}
          placeholder=""
          value={inputPage}
          onChange={(e) => setInputPage(e.target.value)}
          onKeyDown={handleKeyPress}
        />
        <Button isIconOnly onPress={handleJump}>
          Go
        </Button>
      </div>
    </div>
  );
}
