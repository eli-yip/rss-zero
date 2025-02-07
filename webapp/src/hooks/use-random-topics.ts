import { useCallback, useEffect, useState } from "react";

import { apiUrl } from "@/config/config";
import { Topic } from "@/types/topic";

interface Request {
  author: string;
  count: number;
  platform: string;
  type: string;
}

interface Response {
  topics: Topic[];
}

export function useRandomTopics() {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [loading, setLoading] = useState(false);
  const [firstFetch, setFirstFetch] = useState(true);

  const fetchTopics = useCallback(async () => {
    setLoading(true);

    const requestBody: Request = {
      platform: "zhihu",
      type: "answer",
      author: "canglimo",
      count: 10,
    };

    try {
      const response = await fetch(`${apiUrl}/archive/random`, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify(requestBody),
      });

      const data: Response = await response.json();

      setTopics(data.topics);
    } finally {
      setLoading(false);
      setFirstFetch(false);
    }
  }, []);

  useEffect(() => {
    fetchTopics();
  }, [fetchTopics]);

  return { topics, loading, firstFetch, fetchTopics };
}
