import { ActivityCalendar, Activity } from "react-activity-calendar";

import { useTheme } from "@/context/theme-context";

interface StatisticsProps {
  statistics: Record<string, number>;
  loading: boolean;
}

export function Statistics({ loading, statistics }: StatisticsProps) {
  const { state } = useTheme();

  const calLevel = (count: number): number => {
    if (count > 0 && count < 3) {
      return 1;
    } else if (count >= 3 && count < 8) {
      return 2;
    } else if (count >= 8 && count < 12) {
      return 3;
    } else {
      return 4;
    }
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
    <div>
      <ActivityCalendar
        colorScheme={state.theme}
        data={buildData()}
        loading={loading}
        theme={{
          light: ["hsl(0, 0%, 92%)", "rebeccapurple"],
          dark: ["hsl(0, 0%, 22%)", "hsl(225,92%,77%)"],
        }}
      />
    </div>
  );
}
