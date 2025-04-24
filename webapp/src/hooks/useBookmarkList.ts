import { TimeBy, fetchBookmarkList } from "@/api/client";
import type { Topic } from "@/types/Topic";
import { useEffect, useState } from "react";

export function useBookmarkList(
  page: number,
  tag_include: string[] | null = null,
  tag_exclude: string[] | null = null,
  no_tag = false,
  start_date = "",
  end_date = "",
  date_by: TimeBy = TimeBy.Create,
  order = 0,
  order_by: TimeBy = TimeBy.Create,
) {
  const [topics, setTopics] = useState<Topic[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [firstFetchDone, setFirstFetchDone] = useState<boolean>(false);
  const [loading, setLoading] = useState<boolean>(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    async function getBookmarks() {
      setLoading(true);
      setError(null);
      setTopics([]);
      try {
        const data = await fetchBookmarkList(
          page,
          tag_include,
          tag_exclude,
          no_tag,
          start_date,
          end_date,
          date_by,
          order,
          order_by,
        );
        const respData = data.data;
        if (!respData) {
          throw new Error("获取书签列表失败");
        }
        setTopics(respData.topics);
        setTotal(respData.paging.total);
      } catch (e) {
        setError(`加载书签列表失败${e}`);
      } finally {
        setFirstFetchDone(true);
        setLoading(false);
      }
    }
    getBookmarks();
  }, [
    page,
    tag_include,
    tag_exclude,
    no_tag,
    start_date,
    end_date,
    date_by,
    order,
    order_by,
  ]);

  return { topics, total, firstFetchDone, loading, error };
}
