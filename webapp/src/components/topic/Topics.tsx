import { updateBookmark } from "@/api/client";
import { TopicCard } from "@/components/topic/TopicCard";
import { useAllTags } from "@/hooks/useAllTags";
import type { Custom, Topic as TopicType } from "@/types/Topic";
import { addToast } from "@heroui/react";
import { useCallback } from "react";

interface TopicsProps {
  topics: TopicType[];
  onlyBookmark?: boolean;
  onTopicsChange: (updatedTopics: TopicType[]) => void;
}

export function Topics({
  topics,
  onlyBookmark = false,
  onTopicsChange,
}: TopicsProps) {
  // 获取所有标签以便刷新
  const { refetch } = useAllTags();

  // 根据 onlyBookmark 筛选主题
  const filterTopics = (topics: TopicType[]) => {
    if (onlyBookmark) {
      const newTopics = topics.filter(
        (topic) => topic.custom && topic.custom.bookmark === true,
      );
      return newTopics;
    }
    return topics;
  };

  const filteredTopics = filterTopics(topics);

  const handleBookmarkChange = useCallback(
    (topicId: string, isBookmarked: boolean, bookmarkId?: string) => {
      if (!onTopicsChange) return;

      const updatedTopics = topics.map((topic) => {
        if (topic.id === topicId) {
          let custom = null;

          if (isBookmarked) {
            custom = topic.custom ? { ...topic.custom } : ({} as Custom);
            custom.bookmark = true;

            if (bookmarkId) {
              custom.bookmark_id = bookmarkId;
            }
          }

          return {
            ...topic,
            custom,
          };
        }
        return topic;
      });

      onTopicsChange(updatedTopics);
    },
    [topics, onTopicsChange],
  );

  const handleBookmarkDataUpdate = useCallback(
    async (
      topicId: string,
      bookmarkId: string,
      data: { tags?: string[] | null; comment?: string | null },
      type: "tags" | "comment",
    ) => {
      try {
        const tagsToUpdate = data.tags ?? null;
        const commentToUpdate = data.comment ?? null;
        // 传递对应的参数给 API
        await updateBookmark(bookmarkId, null, tagsToUpdate, commentToUpdate);

        if (type === "tags") {
          // 仅在更新标签时刷新标签数据
          await refetch();
        }

        const updatedTopics = topics.map((topic) => {
          if (topic.id === topicId) {
            const custom = topic.custom ? { ...topic.custom } : ({} as Custom);

            // 根据更新类型设置相应属性
            if (tagsToUpdate) custom.tags = tagsToUpdate;
            if (commentToUpdate || commentToUpdate === "")
              custom.comment = commentToUpdate;

            return {
              ...topic,
              custom,
            };
          }
          return topic;
        });

        onTopicsChange(updatedTopics);

        // 根据类型显示不同的成功消息
        addToast({
          title: type === "tags" ? "标签更新成功" : "备注更新成功",
          timeout: 3000,
        });
      } catch (error) {
        console.error(`${type === "tags" ? "标签" : "备注"}更新失败`, error);
        addToast({
          title: `${type === "tags" ? "标签" : "备注"}更新失败`,
          timeout: 3000,
          color: "danger",
        });
      }
    },
    [topics, onTopicsChange, refetch],
  );

  return (
    <div className="mx-auto flex w-full max-w-3xl flex-col gap-y-4">
      {filteredTopics.map((topic) => (
        <TopicCard
          key={topic.id}
          topic={topic}
          onBookmarkChange={handleBookmarkChange}
          onBookmarkDataChange={handleBookmarkDataUpdate}
        />
      ))}
    </div>
  );
}
