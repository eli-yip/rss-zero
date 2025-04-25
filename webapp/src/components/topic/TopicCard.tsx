import {
  Button,
  ButtonGroup,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  Chip,
  Dropdown,
  DropdownItem,
  DropdownMenu,
  DropdownTrigger,
  Input,
  Tooltip,
  addToast,
} from "@heroui/react";
import { DateTime } from "luxon";
import { useCallback, useEffect, useRef, useState } from "react";
import {
  FaArchive,
  FaBookmark,
  FaComment,
  FaCopy,
  FaEllipsisV,
  FaRegBookmark,
  FaSave,
  FaTags,
  FaZhihu,
} from "react-icons/fa";

import { addBookmark, removeBookmark } from "@/api/client";
import { Markdown } from "@/components/topic/Markdown";
import { TagInputForm } from "@/components/topic/TagInput";
import { useAllTags } from "@/hooks/useAllTags";
import { useUserInfo } from "@/hooks/useUserInfo";
import type { Topic } from "@/types/Topic";

interface TopicCardProps {
  topic: Topic;
  onBookmarkChange: (
    topicId: string,
    isBookmarked: boolean,
    bookmarkId?: string, // when deleting a bookmark, this is null
  ) => void;
  onBookmarkDataChange: (
    topicId: string,
    bookmarkId: string,
    data: { tags?: string[] | null; comment?: string | null },
    type: "tags" | "comment",
  ) => void;
}

interface BookmarkedCardBodyProps {
  topic: Topic;
  bookmarkId: string; // 添加 bookmarkId 作为必须属性
  onBookmarkDataChange: (
    topicId: string,
    bookmarkId: string,
    data: { tags?: string[] | null; comment?: string | null },
    type: "tags" | "comment",
  ) => void;
}

// 常规 CardBody 属性
interface RegularCardBodyProps {
  topic: Topic;
}

/**
 * 收藏版文章内容组件
 */
function BookmarkedCardBody({
  topic,
  bookmarkId,
  onBookmarkDataChange,
}: BookmarkedCardBodyProps) {
  // 标签编辑状态
  const [isEditingTags, setIsEditingTags] = useState(false);
  // 备注编辑状态
  const [isEditingComment, setIsEditingComment] = useState(false);
  // 备注内容
  const [commentText, setCommentText] = useState(topic.custom?.comment || "");
  // 使用自定义 hook 获取所有标签
  const { tags, isLoading } = useAllTags();
  // 添加输入框引用
  const commentInputRef = useRef<HTMLInputElement>(null);

  // 将标签数据转换为 TagInputForm 所需的格式
  const tagCountMap = Object.fromEntries(
    tags.map((tag) => [tag.name, tag.count]),
  );

  // 当前标签
  const currentTags = topic.custom?.tags || [];

  // 处理更新标签
  const handleUpdateTag = useCallback(() => {
    // 切换标签编辑状态
    setIsEditingTags((prev) => !prev);
  }, []);

  // 处理标签提交
  const handleTagSubmit = useCallback(
    (newTags: string[]) => {
      onBookmarkDataChange(topic.id, bookmarkId, { tags: newTags }, "tags");
      setIsEditingTags(false);
    },
    [topic.id, bookmarkId, onBookmarkDataChange],
  );

  // 处理更新备注
  const handleUpdateComment = useCallback(() => {
    // 切换评论备注状态
    setIsEditingComment((prev) => !prev);
    if (!isEditingComment) {
      setCommentText(topic.custom?.comment || "");
    }
  }, [isEditingComment, topic.custom]);

  // 处理备注提交
  const handleCommentSubmit = useCallback(() => {
    onBookmarkDataChange(
      topic.id,
      bookmarkId,
      { comment: commentText },
      "comment",
    );
    setIsEditingComment(false);
  }, [topic.id, bookmarkId, commentText, onBookmarkDataChange]);

  // 在编辑状态改变时聚焦输入框
  useEffect(() => {
    if (isEditingComment && commentInputRef.current) {
      commentInputRef.current.focus();
    }
  }, [isEditingComment]);

  return (
    <CardBody>
      {/* 备注显示与编辑 */}
      <div className="mb-2 flex gap-2">
        {isEditingComment ? (
          <div className="flex w-full gap-4">
            <Input
              size="sm"
              value={commentText}
              onValueChange={setCommentText}
              placeholder="添加备注"
              fullWidth
              className="flex-1"
              ref={commentInputRef} // 添加引用
            />
            <Button
              size="sm"
              disableAnimation
              onPress={handleCommentSubmit}
              className="flex-2"
            >
              保存备注
            </Button>
            <Button
              size="sm"
              disableAnimation
              onPress={handleUpdateComment}
              className="flex-2"
            >
              取消编辑
            </Button>
          </div>
        ) : (
          <div className="flex flex-wrap gap-2">
            {Boolean(topic.custom?.comment) && (
              <Chip className="my-auto" size="lg" color="primary">
                {topic.custom?.comment}
              </Chip>
            )}
            <Button
              size="sm"
              disableAnimation
              startContent={<FaComment />}
              onPress={handleUpdateComment}
            >
              编辑备注
            </Button>
          </div>
        )}
      </div>

      {/* 标签编辑模式 */}
      {isEditingTags ? (
        <div className="mb-4">
          {isLoading ? (
            <div>加载标签中...</div>
          ) : (
            <TagInputForm
              tagCountMap={tagCountMap}
              value={currentTags}
              onChange={handleTagSubmit}
              placeholder="输入标签，以空格分隔"
              submitButtonText="保存标签"
            />
          )}
        </div>
      ) : (
        <div className="mb-2 flex flex-wrap gap-2">
          {Boolean(topic.custom?.tags?.length) && (
            <div className="flex flex-wrap gap-2">
              {topic.custom?.tags.map((tag) => (
                <Chip className="my-auto" key={tag} size="lg">
                  {tag}
                </Chip>
              ))}
            </div>
          )}

          <Button size="sm" startContent={<FaTags />} onPress={handleUpdateTag}>
            {isEditingTags ? "取消编辑" : "更新标签"}
          </Button>
        </div>
      )}

      {/* 文章内容 */}
      <Markdown content={topic.body} />
    </CardBody>
  );
}

/**
 * 常规文章内容组件
 */
function RegularCardBody({ topic }: RegularCardBodyProps) {
  return (
    <CardBody>
      <Markdown content={topic.body} />
    </CardBody>
  );
}

/**
 * 文章卡片组件，显示单篇文章的内容
 */
export function TopicCard({
  topic,
  onBookmarkChange,
  onBookmarkDataChange,
  // onTagUpdate,
  // onCommentUpdate,
}: TopicCardProps) {
  const archiveUrl = useArchiveUrl(topic.archive_url);
  const isBookmarked = topic.custom?.bookmark || false;
  const bookmarkId = topic.custom?.bookmark_id || "";

  const { userInfo } = useUserInfo();

  // 处理收藏/取消收藏
  const handleToggleBookmark = useCallback(async () => {
    if (isBookmarked && bookmarkId) {
      await removeBookmark(bookmarkId);
      addToast({
        title: "取消收藏成功",
        timeout: 3000,
      });
      onBookmarkChange?.(topic.id, false);
    } else {
      const result = await addBookmark(topic.type, topic.id);
      addToast({
        title: "添加收藏成功",
        timeout: 3000,
      });
      onBookmarkChange?.(topic.id, true, result.data.bookmark_id);
    }
  }, [isBookmarked, bookmarkId, topic.id, topic.type, onBookmarkChange]);

  const handleCopyMarkdown = useCallback(() => {
    navigator.clipboard.writeText(`# ${topic.title}\n\n${topic.body}`);
    addToast({
      title: "复制全文 Markdown 成功",
      timeout: 3000,
    });
  }, [topic.title, topic.body]);

  const handleCopyArchiveUrl = useCallback(() => {
    navigator.clipboard.writeText(archiveUrl);
    addToast({
      title: "复制存档链接成功",
      timeout: 3000,
    });
  }, [archiveUrl]);

  const handleOpenOriginal = useCallback(() => {
    window.open(topic.original_url, "_blank");
  }, [topic.original_url]);

  const handleOpenArchive = useCallback(() => {
    window.open(archiveUrl, "_blank");
  }, [archiveUrl]);

  const handleAction = useCallback(
    (key: React.Key) => {
      switch (key) {
        case "original":
          handleOpenOriginal();
          break;
        case "archive":
          handleOpenArchive();
          break;
        case "copyMarkdown":
          handleCopyMarkdown();
          break;
        case "copyArchiveUrl":
          handleCopyArchiveUrl();
          break;
        default:
          break;
      }
    },
    [
      handleOpenOriginal,
      handleOpenArchive,
      handleCopyMarkdown,
      handleCopyArchiveUrl,
    ],
  );

  return (
    <Card disableAnimation>
      <CardHeader className="flex justify-between gap-1">
        <p className="mt-2 font-bold text-2xl">{topic.title}</p>
        <div className="mt-2 flex flex-row justify-start sm:justify-end">
          <ButtonGroup>
            <Tooltip content={isBookmarked ? "取消收藏" : "收藏"}>
              <Button
                isIconOnly
                isDisabled={userInfo?.username === "mojia"}
                size="sm"
                onPress={handleToggleBookmark}
              >
                {isBookmarked ? (
                  <FaBookmark size={14} />
                ) : (
                  <FaRegBookmark size={14} />
                )}
              </Button>
            </Tooltip>
            <Dropdown>
              <DropdownTrigger>
                <Button isIconOnly size="sm">
                  <FaEllipsisV size={14} />
                </Button>
              </DropdownTrigger>
              <DropdownMenu onAction={handleAction}>
                <DropdownItem
                  key="original"
                  startContent={<FaZhihu />}
                  description="打开原文链接"
                >
                  原文
                </DropdownItem>
                <DropdownItem
                  key="archive"
                  startContent={<FaArchive />}
                  description="打开存档链接"
                >
                  存档
                </DropdownItem>
                <DropdownItem
                  key="copyMarkdown"
                  startContent={<FaCopy />}
                  description="复制文章完整内容"
                >
                  复制全文
                </DropdownItem>
                <DropdownItem
                  key="copyArchiveUrl"
                  startContent={<FaSave />}
                  description="复制存档链接地址"
                >
                  复制存档链接
                </DropdownItem>
              </DropdownMenu>
            </Dropdown>
          </ButtonGroup>
        </div>
      </CardHeader>

      {/* 根据收藏状态渲染不同的 CardBody */}
      {isBookmarked ? (
        <BookmarkedCardBody
          topic={topic}
          bookmarkId={bookmarkId}
          onBookmarkDataChange={onBookmarkDataChange}
        />
      ) : (
        <RegularCardBody topic={topic} />
      )}

      <CardFooter className="flex flex-col justify-between gap-2 font-bold sm:flex-row">
        <span>{topic.author.nickname}</span>
        <span>{formatDateTime(topic.created_at)}</span>
      </CardFooter>
    </Card>
  );
}

/**
 * 根据当前域名调整存档 URL
 */
function useArchiveUrl(originalArchiveUrl: string): string {
  if (window.location.hostname === "mo.darkeli.com") {
    return originalArchiveUrl.replace("rss-zero", "mo");
  }
  return originalArchiveUrl;
}

/**
 * 格式化日期时间
 */
function formatDateTime(isoString: string): string {
  return DateTime.fromISO(isoString).toFormat("yyyy 年 L 月 d 日 h:mm");
}
