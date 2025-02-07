import { useEffect, useState } from "react";

import { ArchiveResponse, fetchArchiveTopics } from "@/api/archive";
import { Topic } from "@/types/topic";

export function useArchiveTopics(
  page: number,
  startDate: string = "",
  endDate: string = "",
) {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [firstFetchDone, setFirstFetchDone] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function getTopics() {
      setLoading(true);
      setError(null);
      try {
        const data: ArchiveResponse = await fetchArchiveTopics(
          page,
          startDate,
          endDate,
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
  }, [page, startDate, endDate]);

  return { topics, total, firstFetchDone, loading, error };
}
