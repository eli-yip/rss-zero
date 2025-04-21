import {
  type ArchiveResponse,
  ContentType,
  fetchArchiveTopics,
} from "@/api/client";
import type { Topic } from "@/types/topic";
import { useEffect, useState } from "react";

export function useArchiveTopics(
  page: number,
  startDate = "",
  endDate = "",
  contentType: ContentType = ContentType.Answer,
  author = "canglimo",
  order = 0,
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
          contentType,
          author,
          order,
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
  }, [page, order, startDate, endDate, contentType, author]);

  return { topics, total, firstFetchDone, loading, error };
}
