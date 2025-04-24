import { Button } from "@heroui/react";
import { useCallback, useEffect, useState } from "react";

import { ScrollToTop } from "@/components/scroll-to-top";
import { Topics } from "@/components/topic/Topics";
import { useRandomTopics } from "@/hooks/use-random-topics";
import DefaultLayout from "@/layouts/default";
import type { Topic } from "@/types/Topic";

export default function RandomPage() {
  const {
    topics: initialTopics,
    loading,
    firstFetch,
    getTopics,
  } = useRandomTopics();

  // 添加本地状态管理 topics
  const [topics, setTopics] = useState<Topic[]>([]);

  // 当初始数据变化时更新本地状态
  useEffect(() => {
    setTopics(initialTopics);
  }, [initialTopics]);

  // 处理收藏状态变化的回调函数
  const handleTopicsChange = useCallback((updatedTopics: Topic[]) => {
    setTopics(updatedTopics);
  }, []);

  const button = (
    <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
      <Button
        className="font-bold text-2xl"
        isLoading={loading}
        size="lg"
        onPress={getTopics}
      >
        再来一打
      </Button>
    </section>
  );

  return (
    <DefaultLayout>
      {button}
      <Topics topics={topics} onTopicsChange={handleTopicsChange} />
      <ScrollToTop />
      {!firstFetch && button}
    </DefaultLayout>
  );
}
