import { Button } from "@heroui/react";

import DefaultLayout from "@/layouts/default";
import { Topics } from "@/components/topic";
import { ScrollToTop } from "@/components/scroll-to-top";
import { useRandomTopics } from "@/hooks/use-random-topics";

export default function RandomPage() {
  const { topics, loading, firstFetch, fetchTopics } = useRandomTopics();

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
      <ScrollToTop />
      {!firstFetch && button}
    </DefaultLayout>
  );
}
