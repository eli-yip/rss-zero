import { useCallback, useEffect, useState } from "react";

import { type RandomResponse, fetchRandomTopics } from "@/api/client";
import type { Topic } from "@/types/Topic";

export function useRandomTopics() {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const getTopics = useCallback(async () => {
    setLoading(true);
    setTopics([]);
    setError(null);
    try {
      const response: RandomResponse = await fetchRandomTopics();

      setTopics(response.topics);
    } catch (e) {
      setError(`加载数据失败${e}`);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    getTopics();
  }, [getTopics]);

  return { topics, loading, error, getTopics };
}
