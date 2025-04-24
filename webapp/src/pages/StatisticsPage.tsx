import { useEffect, useState } from "react";

import { fetchStatistics } from "@/api/client";
import { title } from "@/components/primitives";
import { Statistics } from "@/components/Statistics";
import DefaultLayout from "@/layouts/default";

export default function StaticsPage() {
  const [statistics, setStatistics] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    fetchStatistics().then((data) => {
      setStatistics(data);
      setLoading(false);
    });
  }, []);

  return (
    <DefaultLayout>
      <section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
        <div className="inline-block max-w-lg justify-center text-center">
          <h1 className={title()}>回答热力图</h1>
        </div>
      </section>

      <div className="flex justify-center align-middle">
        <Statistics loading={loading} statistics={statistics} />
      </div>
    </DefaultLayout>
  );
}
