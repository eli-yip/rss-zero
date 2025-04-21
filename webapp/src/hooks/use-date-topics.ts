import { useEffect, useState } from "react";

import { type ArchiveResponse, fetchArchiveTopics } from "@/api/client";
import type { Topic } from "@/types/topic";

export function useDateTopics(page: number, date: string) {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [firstFetchDone, setFirstFetchDone] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function getTopics() {
      if (!date) {
        return;
      }

      setLoading(true);
      setError(null);
      try {
        const data: ArchiveResponse = await fetchArchiveTopics(
          page,
          date,
          date,
        );

        setTopics(data.topics);
        setTotal(data.paging.total);
      } catch (e) {
        setError(`加载数据失败${e}`);
      } finally {
        setFirstFetchDone(true);
        setLoading(false);
      }
    }
    getTopics();
  }, [page, date]);

  return { topics, total, firstFetchDone, loading, error };
}
