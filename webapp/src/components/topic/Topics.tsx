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
      console.log("new topics", newTopics);
      console.log("new topics lenght", newTopics.length);
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

  // 处理标签更新
  const handleTagUpdate = useCallback(
    async (topicId: string, bookmarkId: string, tags: string[]) => {
      try {
        await updateBookmark(bookmarkId, null, tags, null);
        // 刷新标签数据
        await refetch();

        const updatedTopics = topics.map((topic) => {
          if (topic.id === topicId) {
            const custom = topic.custom ? { ...topic.custom } : ({} as Custom);
            custom.tags = tags;

            return {
              ...topic,
              custom,
            };
          }
          return topic;
        });

        onTopicsChange(updatedTopics);

        addToast({
          title: "标签更新成功",
          timeout: 3000,
        });
      } catch (error) {
        console.error("更新标签失败", error);
        addToast({
          title: "标签更新失败",
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
          onTagUpdate={handleTagUpdate}
        />
      ))}
    </div>
  );
}
