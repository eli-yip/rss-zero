import { useEffect, useState } from "react";
import { Button } from "@heroui/react";

import DefaultLayout from "@/layouts/default";
import { Topics } from "@/components/topic";
import { apiUrl } from "@/config/config";
import { Topic } from "@/types/topic";

export default function RandomPage() {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [loading, setLoading] = useState(false);

  interface Request {
    author: string;
    count: number;
    platform: string;
    type: string;
  }
  interface Response {
    topics: Topic[];
  }

  const fetchTopics = async () => {
    setLoading(true);

    const requestBody: Request = {
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

    const data: Response = await response.json();

    setTopics(data.topics);
    setLoading(false);

    window.scrollTo(0, 0);
  };

  useEffect(() => {
    fetchTopics();
  }, []);

  const button = (
    <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
      <Button
        className="text-2xl font-bold"
        isLoading={loading}
        size="lg"
        onPress={fetchTopics}
      >
        再来一打
      </Button>
    </section>
  );

  return (
    <DefaultLayout>
      {button}
      <Topics topics={topics} />
      {button}
    </DefaultLayout>
  );
}
