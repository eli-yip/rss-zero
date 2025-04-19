import { Tooltip } from "@heroui/react";
import { type Activity, ActivityCalendar } from "react-activity-calendar";
import { useSearchParams } from "react-router-dom";

import { Archive } from "@/components/archive";
import { useTheme } from "@/context/theme-context";
import { useDateTopics } from "@/hooks/use-date-topics";

interface StatisticsProps {
  statistics: Record<string, number>;
  loading: boolean;
}

export function Statistics({ loading, statistics }: StatisticsProps) {
  const { state } = useTheme();
  const [searchParams, setSearchParams] = useSearchParams();
  const page = Number.parseInt(searchParams.get("page") || "1");
  const date = searchParams.get("date") || "";
  const { topics, total, loading: topicsLoading } = useDateTopics(page, date);

  const calLevel = (count: number): number => {
    if (count > 0 && count < 3) {
      return 1;
    }
    if (count >= 3 && count < 8) {
      return 2;
    }
    if (count >= 8 && count < 12) {
      return 3;
    }
    return 4;
  };

  const buildData = (): Array<Activity> => {
    return Object.entries(statistics).map(([date, count]): Activity => {
      return {
        date,
        count,
        level: calLevel(count),
      };
    });
  };

  return (
    <div className="max-w-full">
      <div className="mb-4 flex justify-center">
        <ActivityCalendar
          blockSize={16}
          colorScheme={state.theme}
          data={buildData()}
          eventHandlers={{
            onClick: () => (activity: Activity) => {
              setSearchParams(new URLSearchParams({ date: activity.date }));
            },
          }}
          labels={{ totalCount: "过去一年写下 {{count}} 篇文章" }}
          loading={loading}
          renderBlock={(block, activity) => (
            <Tooltip
              closeDelay={100}
              content={`${activity.date} 写下 ${activity.count} 篇文章`}
              offset={15}
            >
              {block}
            </Tooltip>
          )}
          theme={{
            light: ["hsl(0, 0%, 92%)", "rebeccapurple"],
            dark: ["hsl(0, 0%, 22%)", "hsl(225,92%,77%)"],
          }}
        />
      </div>
      {!topicsLoading && topics && (
        <Archive
          firstFetchDone={true}
          loading={loading}
          page={page}
          setSearchParams={setSearchParams}
          topics={topics}
          total={total}
        />
      )}
    </div>
  );
}
