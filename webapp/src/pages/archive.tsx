import { DatePicker, type DateValue, Select, SelectItem } from "@heroui/react";
import { parseDate } from "@internationalized/date";
import { useSearchParams } from "react-router-dom";

import { ContentType } from "@/api/archive";
import { Archive } from "@/components/archive";
import { title } from "@/components/primitives";
import { useArchiveTopics } from "@/hooks/use-archive-topics";
import DefaultLayout from "@/layouts/default";
import { scrollToTop } from "@/utils/window";

const canglimoUrlToken: string = "canglimo";
const authors = [
	{ key: canglimoUrlToken, name: "墨苍离" },
	{ key: "zi-e-79-23", name: "别打扰我修道" },
];

export default function ArchivePage() {
	const [searchParams, setSearchParams] = useSearchParams();
	const pageStr = searchParams.get("page") || "1";
	const page = Number.parseInt(pageStr);
	const startDate = searchParams.get("startDate") || "";
	const endDate = searchParams.get("endDate") || "";
	const author = searchParams.get("author") || canglimoUrlToken;
	const isValidContentType = (value: string | null): value is ContentType => {
		return (
			value !== null &&
			Object.values(ContentType).includes(value as ContentType)
		);
	};
	const contentType = isValidContentType(searchParams.get("contentType"))
		? (searchParams.get("contentType") as ContentType)
		: ContentType.Answer;
	const { topics, total, firstFetchDone, loading } = useArchiveTopics(
		page,
		startDate,
		endDate,
		contentType,
		author,
	);

	const updateDateParam = (param: string, value: DateValue | null) => {
		const dateValue = value ? value.toString() : "";

		searchParams.set("page", "1");
		if (dateValue) searchParams.set(param, dateValue);
		else searchParams.delete(param);

		setSearchParams(searchParams);
		scrollToTop();
	};

	const handleContentTypeChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
		searchParams.set("contentType", e.target.value);
		searchParams.set("page", "1");
		setSearchParams(searchParams);
		scrollToTop();
	};

	const handleStartDateChange = (value: DateValue | null) => {
		updateDateParam("startDate", value);
	};

	const handleEndDateChange = (value: DateValue | null) => {
		updateDateParam("endDate", value);
	};

	const handleAuthorChange = (e: React.ChangeEvent<HTMLSelectElement>) => {
		searchParams.set("author", e.target.value);
		searchParams.set("page", "1");
		setSearchParams(searchParams);
		scrollToTop();
	};

	return (
		<DefaultLayout>
			<section className="flex flex-col items-center justify-center gap-4 py-8 md:py-10">
				<div className="inline-block max-w-lg justify-center text-center">
					<h1 className={title()}>历史文章</h1>
				</div>
			</section>

			<div className="mx-auto flex w-full max-w-xs flex-col pb-4">
				<div className="mx-auto my-0 flex w-full gap-4">
					<DatePicker
						showMonthAndYearPickers
						label="开始时间"
						value={startDate ? parseDate(startDate) : null}
						onChange={handleStartDateChange}
					/>
					<DatePicker
						showMonthAndYearPickers
						label="截止时间"
						value={endDate ? (parseDate(endDate) as DateValue) : null}
						onChange={handleEndDateChange}
					/>
				</div>
				<div className="mx-auto my-4 flex w-full gap-4">
					<Select
						defaultSelectedKeys={[contentType]}
						value={contentType}
						onChange={handleContentTypeChange}
					>
						<SelectItem key={ContentType.Answer}>回答</SelectItem>
						<SelectItem key={ContentType.Pin}>想法</SelectItem>
					</Select>
					<Select
						defaultSelectedKeys={[author]}
						value={author}
						onChange={handleAuthorChange}
					>
						{authors.map((item) => (
							<SelectItem key={item.key}>{item.name}</SelectItem>
						))}
					</Select>
				</div>
			</div>

			<Archive
				firstFetchDone={firstFetchDone}
				loading={loading}
				page={page}
				setSearchParams={setSearchParams}
				topics={topics}
				total={total}
			/>
		</DefaultLayout>
	);
}
