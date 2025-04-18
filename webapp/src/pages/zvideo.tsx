import { useCallback, useEffect, useState } from "react";

import { title } from "@/components/primitives";
import { ScrollToTop } from "@/components/scroll-to-top";
import { Zvideos } from "@/components/zvideos";
import { apiUrl } from "@/config/config";
import DefaultLayout from "@/layouts/default";
import type { Zvideo } from "@/types/zvideo";

export default function ZvideoPage() {
	const [zvideos, setZvideos] = useState<Zvideo[]>();

	interface ZvideoResponse {
		zvideos: Zvideo[];
	}

	const getZvideos = useCallback(async () => {
		const response = await fetch(`${apiUrl}/archive/zvideo`);

		if (!response.ok) {
			throw new Error("Failed to fetch zvideos");
		}

		const data: ZvideoResponse = await response.json();

		return data.zvideos;
	}, []);

	useEffect(() => {
		async function fetchData() {
			const zvideos = await getZvideos();

			setZvideos(zvideos);
		}

		fetchData();
	}, [getZvideos]);

	return (
		<DefaultLayout>
			<section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
				<div className="inline-block max-w-lg justify-center text-center">
					<h1 className={title()}>直播回放</h1>
				</div>
			</section>

			{zvideos && (
				<div>
					<ScrollToTop />
					<div className="flex justify-center align-middle">
						<Zvideos zvideos={zvideos} />
					</div>
				</div>
			)}
		</DefaultLayout>
	);
}
