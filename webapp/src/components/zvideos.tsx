import { Card, CardBody, Link } from "@heroui/react";
import { DateTime } from "luxon";

import type { Zvideo } from "@/types/zvideo";

interface ZvideosProps {
	zvideos: Zvideo[];
}

export function Zvideos({ zvideos }: ZvideosProps) {
	const parseDate = (date: string): string => {
		const beijingDate = DateTime.fromISO(date, { zone: "Asia/Shanghai" });

		return beijingDate.toFormat("yyyy 年 L 月 d 日");
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
