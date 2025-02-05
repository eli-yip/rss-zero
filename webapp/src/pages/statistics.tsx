import { useState, useEffect } from "react";

import { title } from "@/components/primitives";
import DefaultLayout from "@/layouts/default";
import { Statistics } from "@/components/statistics";
import { fetchStatistics } from "@/api/statistics";

export default function DocsPage() {
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
          <h1 className={title()}>虚假的数据</h1>
        </div>
      </section>

      <div className="flex justify-center align-middle">
        <Statistics loading={loading} statistics={statistics} />
      </div>
    </DefaultLayout>
  );
}
