import { useCallback, useEffect, useState } from "react";

import { type RandomResponse, fetchRandomTopics } from "@/api/client";
import type { Topic } from "@/types/Topic";

export function useRandomTopics() {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [firstFetchDone, setFirstFetchDone] = useState(true);

  const getTopics = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const response: RandomResponse = await fetchRandomTopics();

      setTopics(response.topics);
    } catch (e) {
      setError(`加载数据失败${e}`);
    } finally {
      setLoading(false);
      setFirstFetchDone(true);
    }
  }, []);

  useEffect(() => {
    getTopics();
  }, [getTopics]);

  return { topics, loading, error, firstFetch: firstFetchDone, getTopics };
}
