import { Card, CardBody, Link } from "@heroui/react";
import moment from "moment-timezone";

import { Zvideo } from "@/types/zvideo";

interface ZvideosProps {
  zvideos: Zvideo[];
}

export function Zvideos({ zvideos }: ZvideosProps) {
  const parseDate = (date: string): string => {
    const beijingDate = moment.tz(date, "Asia/Shanghai");

    return beijingDate.format("YYYY年MM月DD日");
  };

  return (
    <div className="w-full max-w-3xl gap-4 sm:grid sm:grid-cols-2">
      {zvideos.map((zvideo) => (
        <Card key={zvideo.id} className="mb-4 sm:mb-0">
          <CardBody>
            <div className="flex justify-between gap-4">
              <p>{parseDate(zvideo.published_at)}</p>
              <Link isExternal showAnchorIcon href={zvideo.url} target="_blank">
                {zvideo.title}
              </Link>
            </div>
          </CardBody>
        </Card>
      ))}
    </div>
  );
}
