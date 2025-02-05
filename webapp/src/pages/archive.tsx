import { useSearchParams, useNavigate } from "react-router-dom";
import { useState, useEffect } from "react";

import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";
import { Topics } from "@/components/topic";
import { ScrollToTop } from "@/components/scroll-to-top";
import { scrollToTop } from "@/utils/window";
import PaginationWrapper from "@/components/pagination";
import { useArchiveTopics } from "@/hooks/use-archive-topics";

export default function ArchivePage() {
  const [searchParams] = useSearchParams();
  const pageStr = searchParams.get("page") || "1";
  const page = parseInt(pageStr);
  const navigate = useNavigate();
  const { topics, total, loading } = useArchiveTopics(page);
  const [firstFetch, setFirstFetch] = useState(true);

  const handlePageChange = (page: number) => {
    navigate(`/archive?page=${page}`);
    scrollToTop();
  };

  useEffect(() => {
    setFirstFetch(false);
  }, []);

  return (
    <DefaultLayout>
      <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
        <div className="inline-block max-w-lg justify-center text-center">
          <h1 className={title()}>历史文章</h1>
        </div>
      </section>

      {!loading && <Topics topics={topics} />}

      <ScrollToTop />
      {!firstFetch && (
        <PaginationWrapper
          page={page}
          total={total}
          onChange={handlePageChange}
        />
      )}
    </DefaultLayout>
  );
}
