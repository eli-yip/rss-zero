import { apiUrl } from "@/config/config";
import type { Topic } from "@/types/topic";

interface RandomRequest {
  author: string;
  count: number;
  platform: string;
  type: string;
}

export interface RandomResponse {
  topics: Topic[];
}

export async function fetchRandomTopics() {
  const requestBody: RandomRequest = {
    platform: "zhihu",
    type: "answer",
    author: "canglimo",
    count: 10,
  };

  const response = await fetch(`${apiUrl}/archive/random`, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(requestBody),
  });

  if (!response.ok) {
    throw new Error("请求失败");
  }

  return await response.json();
}
