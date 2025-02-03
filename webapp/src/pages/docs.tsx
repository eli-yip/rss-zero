import { useEffect, useState } from "react";

import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";
import { Topics } from "@/components/topic";
import { apiUrl } from "@/config/config";

export default function DocsPage() {
  const [topics, setTopics] = useState<string[]>([]);

  interface Request {
    author: string;
    count: number;
    platform: string;
    type: string;
  }
  interface Response {
    topics: Topic[];
  }
  interface Topic {
    id: string;
    text: string;
  }

  const fetchTopics = async () => {
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
    const contents = data.topics.map((topic) => topic.text);

    setTopics(contents);
  };

  useEffect(() => {
    fetchTopics();
  }, []);

  return (
    <DefaultLayout>
      <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
        <div className="inline-block max-w-lg text-center justify-center">
          <h1 className={title()}>Docs</h1>
        </div>
      </section>
      <Topics contents={topics} />
    </DefaultLayout>
  );
}
