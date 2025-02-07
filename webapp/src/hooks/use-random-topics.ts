import { useEffect, useState } from "react";

import { fetchRandomTopics, RandomResponse } from "@/api/random";
import { Topic } from "@/types/topic";

export function useRandomTopics() {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [firstFetchDone, setFirstFetchDone] = useState(true);

  async function getTopics() {
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
  }

  useEffect(() => {
    getTopics();
  }, []);

  return { topics, loading, error, firstFetch: firstFetchDone, getTopics };
}
