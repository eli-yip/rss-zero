import { Input } from "@heroui/react";
import { Listbox, ListboxItem } from "@heroui/react";
import { Button, Form } from "@heroui/react";
import { useRef, useState } from "react";
import { useFilter } from "react-aria";

// 定义 listbox 项的接口
interface ListboxItemData {
  key: string;
  label: string;
  count: number;
}

interface TagInputProps {
  // 标签名到数量的映射
  tagCountMap: Record<string, number>;
  // 当前已选标签，以空格分隔的字符串
  value: string;
  // 更新标签的回调函数
  onChange: (value: string) => void;
  // 可选属性
  placeholder?: string;
}

interface wordInfo {
  word: string;
  start: number;
  end: number;
}

// 辅助函数：获取光标位置的当前词
const getCurrentWord = (text: string, cursorPosition: number): wordInfo => {
  // 如果光标在空白处或文本末尾，返回空字符串
  if (cursorPosition > text.length) {
    return { word: "", start: cursorPosition, end: cursorPosition };
  }

  // 查找光标左侧的第一个空格
  let start = text.substring(0, cursorPosition).lastIndexOf(" ");
  if (start === -1) start = 0;
  else start += 1; // 跳过空格

  // 查找光标右侧的第一个空格
  let end = text.indexOf(" ", cursorPosition);
  if (end === -1) end = text.length;

  // 如果光标在空格后面且后面没有内容或是空格
  if (
    start === cursorPosition &&
    (end === cursorPosition || text[cursorPosition] === " ")
  ) {
    return { word: "", start, end: start };
  }

  return {
    word: text.substring(start, end),
    start,
    end,
  };
};

function TagInput({
  tagCountMap,
  value,
  onChange,
  placeholder = "输入标签，以空格分隔",
}: TagInputProps) {
  // 状态和引用
  const [isOpen, setIsOpen] = useState(false);
  const [cursorPosition, setCursorPosition] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);

  // 将标签按照数量降序排序
  const sortedTags = Object.entries(tagCountMap)
    .sort(([, countA], [, countB]) => countB - countA)
    .map(([tag]) => tag);

  // 当前输入的标签数组
  const currentTags = value.trim().split(/\s+/).filter(Boolean);

  // 获取当前光标位置的词
  const currentWordInfo = getCurrentWord(value, cursorPosition);
  const currentWord = currentWordInfo.word;

  // 使用 react-aria 的 useFilter 进行过滤
  const { contains } = useFilter({ sensitivity: "base" });

  // 过滤标签：排除已经使用的标签，并根据当前词进行筛选
  const filteredTags = sortedTags
    .filter((tag) => !currentTags.includes(tag) || tag === currentWord)
    .filter((tag) => currentWord === "" || contains(tag, currentWord));

  // 显示的标签列表，如果当前没有词，则显示前 5 个标签
  const displayTags =
    currentWord === "" ? filteredTags.slice(0, 5) : filteredTags;
  // 转换为 Listbox 需要的格式
  const listboxItems: ListboxItemData[] = displayTags.map((tag) => ({
    key: tag,
    label: tag,
    count: tagCountMap[tag],
  }));

  // 处理输入框变化
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange(e.target.value);
    const position = e.target.selectionStart || 0;
    setCursorPosition(position);
    setTimeout(() => {
      setIsOpen(true);
    });
  };

  // 处理获取焦点
  const handleFocus = (_: React.FocusEvent<HTMLInputElement>) => {
    setTimeout(() => {
      setIsOpen(true);
    });
  };

  const handleSelect = (e: React.FocusEvent<HTMLInputElement>) => {
    if (e.target.selectionStart !== e.target.selectionEnd) {
      return;
    }

    const position = e.target.selectionStart || 0;
    setCursorPosition(position);
    setTimeout(() => {
      setIsOpen(true);
    });
  };

  // 处理标签选择
  const handleTagSelect = (tag: string) => {
    const newValue = `${value.substring(0, currentWordInfo.start)}${tag}${value.substring(currentWordInfo.end)}`;

    onChange(newValue);
    setTimeout(() => {
      if (inputRef.current) {
        inputRef.current.focus();
        setTimeout(() => {
          setIsOpen(false);
        });
        const newPosition = currentWordInfo.start + tag.length;
        inputRef.current.setSelectionRange(newPosition, newPosition);
        setCursorPosition(newPosition);
      }
    });
  };

  const handleBlur = () => {
    setTimeout(() => {
      setIsOpen(false);
    });
  };

  return (
    <div className="relative">
      <Input
        isClearable
        ref={inputRef}
        value={value}
        onChange={handleInputChange}
        onClear={() => onChange("")}
        onFocus={handleFocus}
        onSelect={handleSelect}
        onBlur={handleBlur}
        placeholder={placeholder}
        fullWidth
      />
      {isOpen && displayTags.length > 0 ? (
        <div className="absolute z-50 w-full">
          <Listbox
            label="标签列表"
            items={listboxItems}
            onAction={(key) => handleTagSelect(key.toString())}
            className=" bg-sky-50"
            itemClasses={{
              base: "data-[hover=true]:bg-sky-300 w-full text-nowrap",
            }}
          >
            {listboxItems.map((item) => (
              <ListboxItem key={item.key} textValue={item.label} className="">
                <div className="flex justify-between">
                  <span className="font-bold">{item.label}</span>
                  <span className="text-gray-500">{item.count}</span>
                </div>
              </ListboxItem>
            ))}
          </Listbox>
        </div>
      ) : null}
    </div>
  );
}

// 类型定义
interface TagInputFormProps {
  // 标签名到数量的映射
  tagCountMap: Record<string, number>;
  // 当前已选标签数组
  value: string[];
  // 更新标签的回调函数
  onChange: (value: string[]) => void;
  // 可选属性
  placeholder?: string;
  submitButtonText?: string;
}

export function TagInputForm({
  tagCountMap,
  value,
  onChange,
  placeholder = "输入标签，以空格分隔",
  submitButtonText = "提交",
}: TagInputFormProps) {
  // 内部状态，只在提交时更新到外部
  const [internalValue, setInternalValue] = useState<string>(value.join(" "));

  // 表单提交处理
  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const newTags = internalValue.trim().split(/\s+/).filter(Boolean);
    onChange(newTags);
  };

  return (
    <Form onSubmit={handleSubmit} className="w-full">
      <div className="flex w-full gap-4">
        <div className="flex-1">
          <TagInput
            tagCountMap={tagCountMap}
            value={internalValue}
            onChange={setInternalValue}
            placeholder={placeholder}
          />
        </div>
        <div className="flex-2">
          <Button type="submit">{submitButtonText}</Button>
        </div>
      </div>
    </Form>
  );
}
