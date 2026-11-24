import { createFileRoute, Link } from "@tanstack/react-router";
import {
	type ColumnDef,
	columnFilteringFeature,
	createFilteredRowModel,
	createPaginatedRowModel,
	createSortedRowModel,
	filterFn_includesString,
	rowPaginationFeature,
	rowSortingFeature,
	tableFeatures,
	useTable,
} from "@tanstack/react-table";
import { cn } from "cn";
import { ArrowDown, ArrowUp, ArrowUpDown, Minus } from "lucide-react";
import { useEffect, useMemo } from "react";
import { Line, LineChart, YAxis } from "recharts";
import { Button } from "#/components/ui/button";
import { Input } from "#/components/ui/input";
import {
	Select,
	SelectContent,
	SelectItem,
	SelectTrigger,
	SelectValue,
} from "#/components/ui/select";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "#/components/ui/table";
import type {
	LatestRanksRecord,
	RankHistoryRecord,
} from "#/lib/pocketbase-types";

const RANK_FILTERS = {
	all: { label: "All ranks", test: () => true },
	top10: { label: "Top 10", test: (r: number) => r >= 1 && r <= 10 },
	mid: { label: "11 to 50", test: (r: number) => r >= 11 && r <= 50 },
	deep: { label: "51 to 200", test: (r: number) => r >= 51 },
	unranked: { label: "Not in top 200", test: (r: number) => r === 0 },
} as const;
type RankFilter = keyof typeof RANK_FILTERS;

// Buckets for the standings strip, in display order.
const BUCKETS: Array<{ key: Exclude<RankFilter, "all">; bar: string }> = [
	{ key: "top10", bar: "bg-gold" },
	{ key: "mid", bar: "bg-primary" },
	{ key: "deep", bar: "bg-primary/35" },
	{ key: "unranked", bar: "bg-muted-foreground/20" },
];

const PAGE_SIZES = [10, 25, 50, 100];

const SORT_IDS = ["term", "rank", "change"] as const;
type SortId = (typeof SORT_IDS)[number];
const TREND_DAYS = 30;

const ALL_COUNTRIES = "all";

type IndexSearch = {
	q?: string;
	/** Lower-case storefront code, e.g. "gb". Omitted for all countries. */
	country?: string;
	rank?: Exclude<RankFilter, "all">;
	sort?: SortId;
	desc?: boolean;
	page?: number; // 1-based
	size?: number;
};

export const Route = createFileRoute("/")({
	ssr: false,
	// Omit defaults so a pristine page has a clean URL.
	validateSearch: (s: Record<string, unknown>): IndexSearch => {
		const rank =
			typeof s.rank === "string" && s.rank in RANK_FILTERS && s.rank !== "all"
				? (s.rank as IndexSearch["rank"])
				: undefined;
		const sort = SORT_IDS.find((id) => id === s.sort);
		const page = Number(s.page);
		const size = Number(s.size);
		return {
			q: typeof s.q === "string" && s.q ? s.q : undefined,
			country:
				typeof s.country === "string" && /^[a-z]{2}$/i.test(s.country)
					? s.country.toLowerCase()
					: undefined,
			rank,
			sort: sort && !(sort === "rank" && s.desc !== true) ? sort : undefined,
			desc: s.desc === true || s.desc === "true" ? true : undefined,
			page: Number.isInteger(page) && page > 1 ? page : undefined,
			size: PAGE_SIZES.includes(size) && size !== 25 ? size : undefined,
		};
	},
	loader: async ({ context }) => {
		const { pb } = context;
		const ranks = await pb
			.collection<LatestRanksRecord>("latest_ranks")
			.getFullList({ filter: "is_mine = true" });

		// Daily rank series per keyword for the sparklines (my app only).
		const trends: Record<string, Array<number | null>> = {};
		const appId = ranks[0]?.app ? String(ranks[0].app) : null;
		if (appId) {
			const since = new Date(Date.now() - TREND_DAYS * 86400_000)
				.toISOString()
				.replace("T", " ");
			const history = await pb
				.collection<RankHistoryRecord>("rank_history")
				.getFullList({
					filter: pb.filter("app = {:app} && checked_at >= {:since}", {
						app: appId,
						since,
					}),
					sort: "checked_at",
					fields: "keyword,rank,checked_at",
				});
			// One point per keyword per day (last check wins); 0 -> gap.
			const perDay = new Map<string, Map<string, number | null>>();
			for (const h of history) {
				const days = perDay.get(h.keyword) ?? new Map();
				days.set(h.checked_at.slice(0, 10), h.rank ? h.rank : null);
				perDay.set(h.keyword, days);
			}
			for (const [kw, days] of perDay) trends[kw] = [...days.values()];
		}

		return { ranks: ranks ?? [], trends };
	},
	component: App,
});

function RankBadge({ rank }: { rank: number }) {
	if (!rank)
		return (
			<span className="font-display text-lg font-semibold text-muted-foreground">
				200+
			</span>
		);
	const top = rank <= 10;
	return (
		<span
			className={cn(
				"font-display tnum inline-grid min-w-12 place-items-center rounded-lg px-2 py-0.5 text-xl font-bold leading-tight tracking-tight",
				top ? "bg-gold text-gold-ink" : "text-foreground",
			)}
		>
			{rank}
		</span>
	);
}

function Change({
	c,
}: {
	c: { label: string | null; delta: number | null } | null;
}) {
	const base =
		"tnum inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-xs font-semibold";
	if (!c) return <span className="text-muted-foreground">–</span>;
	if (c.label === "new")
		return (
			<span className={cn(base, "bg-accent text-accent-foreground")}>New</span>
		);
	if (c.label === "dropped")
		return <span className={cn(base, "bg-loss/12 text-loss")}>Fell out</span>;
	const d = c.delta ?? 0;
	if (d === 0)
		return (
			<Minus className="size-4 text-muted-foreground" aria-label="No change" />
		);
	return d > 0 ? (
		<span className={cn(base, "bg-gain/12 text-gain")}>
			<ArrowUp className="size-3" />
			{d}
		</span>
	) : (
		<span className={cn(base, "bg-loss/12 text-loss")}>
			<ArrowDown className="size-3" />
			{-d}
		</span>
	);
}

function Sparkline({ points }: { points: Array<number | null> | undefined }) {
	if (!points || points.filter((p) => p != null).length < 2)
		return <span className="text-muted-foreground">–</span>;
	return (
		<LineChart
			width={96}
			height={28}
			data={points.map((rank) => ({ rank }))}
			margin={{ top: 3, right: 3, bottom: 3, left: 3 }}
			aria-hidden
		>
			{/* Lower rank number = better, so flip the axis: up means improving. */}
			<YAxis hide reversed domain={["dataMin", "dataMax"]} />
			<Line
				type="monotone"
				dataKey="rank"
				stroke="var(--series-1)"
				strokeWidth={1.75}
				dot={false}
				activeDot={false}
				isAnimationActive={false}
				connectNulls={false}
			/>
		</LineChart>
	);
}

function CountryTag({ country }: { country: string }) {
	return (
		<span className="ml-2 rounded bg-muted px-1.5 py-0.5 align-middle text-[10px] font-bold uppercase tracking-wider text-muted-foreground">
			{country}
		</span>
	);
}

type Mover = { row: LatestRanksRecord; delta: number | null; label: string };

function MoversList({
	title,
	tone,
	items,
	empty,
	showCountry,
}: {
	title: string;
	tone: "gain" | "loss";
	items: Mover[];
	empty: string;
	showCountry: boolean;
}) {
	return (
		<div className="card p-5">
			<h2 className="font-display mb-3 text-lg font-bold">{title}</h2>
			{items.length === 0 ? (
				<p className="text-sm text-muted-foreground">{empty}</p>
			) : (
				<ul className="divide-y">
					{items.map(({ row, delta, label }) => (
						<li
							key={row.id}
							className="flex items-center justify-between gap-3 py-2"
						>
							<Link
								to="/keywords/$keywordId"
								params={{ keywordId: String(row.keyword) }}
								className="truncate font-semibold hover:text-primary"
							>
								{row.term}
								{showCountry ? <CountryTag country={row.country} /> : null}
							</Link>
							<span className="tnum flex shrink-0 items-center gap-2 text-sm">
								<span className="text-muted-foreground">{label}</span>
								<span
									className={cn(
										"inline-flex min-w-10 items-center justify-center gap-1 rounded-full px-2 py-0.5 text-xs font-semibold",
										tone === "gain"
											? "bg-gain/12 text-gain"
											: "bg-loss/12 text-loss",
									)}
								>
									{delta == null ? (
										tone === "gain" ? (
											"New"
										) : (
											"Fell out"
										)
									) : (
										<>
											{tone === "gain" ? (
												<ArrowUp className="size-3" />
											) : (
												<ArrowDown className="size-3" />
											)}
											{Math.abs(delta)}
										</>
									)}
								</span>
							</span>
						</li>
					))}
				</ul>
			)}
		</div>
	);
}

function App() {
	const { ranks: allRanks, trends } = Route.useLoaderData();
	const search = Route.useSearch();
	const navigate = Route.useNavigate();

	// Storefronts present in the data, with keyword counts. The filter only
	// appears when there is more than one to choose from.
	const countries = useMemo(() => {
		const counts = new Map<string, number>();
		for (const r of allRanks)
			counts.set(r.country, (counts.get(r.country) ?? 0) + 1);
		return [...counts].sort(([a], [b]) => a.localeCompare(b));
	}, [allRanks]);
	// A stale or mistyped ?country= falls back to all countries.
	const activeCountry =
		countries.length > 1 && countries.some(([c]) => c === search.country)
			? search.country
			: undefined;
	// Everything below (table, distribution strip, movers, counts) follows the filter.
	const data = useMemo(
		() =>
			activeCountry
				? allRanks.filter((r) => r.country === activeCountry)
				: allRanks,
		[allRanks, activeCountry],
	);
	const showCountry = countries.length > 1 && !activeCountry;

	const change = (row: LatestRanksRecord) => {
		const prev = row.rank_7d ?? row.rank_first;

		if (prev == null || typeof row.rank !== "number") return null;
		if (prev === 0 && row.rank === 0) return null;
		if (prev === 0) return { label: "new", delta: null };
		if (row.rank === 0) return { label: "dropped", delta: null };
		return { label: null, delta: (prev as number) - row.rank };
	};

	const features = tableFeatures({
		rowSortingFeature,
		columnFilteringFeature,
		rowPaginationFeature,
		sortedRowModel: createSortedRowModel(),
		filteredRowModel: createFilteredRowModel(),
		paginatedRowModel: createPaginatedRowModel(),
		filterFns: { includesString: filterFn_includesString },
	});

	const columns: Array<ColumnDef<typeof features, LatestRanksRecord>> = [
		{
			accessorKey: "term",
			header: "Keyword",
			filterFn: "includesString",
			cell: ({ row }) => (
				<Link
					to="/keywords/$keywordId"
					params={{ keywordId: String(row.original.keyword) }}
					className="row-link text-base font-semibold hover:text-primary"
				>
					{row.original.term}
					{showCountry ? <CountryTag country={row.original.country} /> : null}
				</Link>
			),
		},
		{
			id: "rank",
			accessorFn: (row) => row.rank || undefined,
			sortUndefined: "last",
			sortDescFirst: false,
			header: "Rank",
			filterFn: (row, _columnId, value: RankFilter) =>
				RANK_FILTERS[value].test(Number(row.original.rank) || 0),
			sortFn: (a, b, columnId) =>
				a.getValue<number>(columnId) - b.getValue<number>(columnId),
			cell: ({ row }) => <RankBadge rank={Number(row.original.rank) || 0} />,
		},
		{
			accessorKey: "rank_7d",
			header: "7 days ago",
			cell: (info) => {
				const v = info.getValue();
				return (
					<span className="tnum text-muted-foreground">
						{v == null ? "–" : v === 0 ? "200+" : `#${v}`}
					</span>
				);
			},
		},
		{
			id: "change",
			accessorFn: (row) => change(row),
			header: "Change",
			cell: ({ getValue }) => (
				<Change c={getValue<ReturnType<typeof change>>()} />
			),
			sortFn: (a, b) =>
				(a.getValue<any>("change")?.delta ?? 0) -
				(b.getValue<any>("change")?.delta ?? 0),
		},
		{
			id: "trend",
			header: "30 days",
			enableSorting: false,
			cell: ({ row }) => (
				<Sparkline points={trends[String(row.original.keyword)]} />
			),
		},
	];

	const table = useTable({
		key: "table",
		features,
		columns,
		data,
		initialState: {
			sorting: [{ id: search.sort ?? "rank", desc: search.desc ?? false }],
			columnFilters: [
				...(search.q ? [{ id: "term", value: search.q }] : []),
				...(search.rank ? [{ id: "rank", value: search.rank }] : []),
			],
			pagination: {
				pageIndex: (search.page ?? 1) - 1,
				pageSize: search.size ?? 25,
			},
		},
		enableSortingRemoval: false,
	});

	// Mirror table state into the URL (replace, so back doesn't step through
	// every keystroke). Defaults are omitted to keep the URL clean.
	const { sorting, columnFilters, pagination } = table.state;
	useEffect(() => {
		const sort = sorting[0];
		const q = columnFilters.find((f) => f.id === "term")?.value as
			| string
			| undefined;
		const rank = columnFilters.find((f) => f.id === "rank")?.value as
			| IndexSearch["rank"]
			| undefined;
		const isDefaultSort = !sort || (sort.id === "rank" && !sort.desc);
		const next: IndexSearch = {
			q: q || undefined,
			country: activeCountry,
			rank: rank || undefined,
			sort: isDefaultSort ? undefined : (sort.id as SortId),
			desc: isDefaultSort || !sort.desc ? undefined : true,
			page: pagination.pageIndex > 0 ? pagination.pageIndex + 1 : undefined,
			size: pagination.pageSize !== 25 ? pagination.pageSize : undefined,
		};
		const same = (Object.keys(next) as Array<keyof IndexSearch>).every(
			(k) => next[k] === search[k],
		);
		if (!same) navigate({ search: next, replace: true });
	}, [sorting, columnFilters, pagination, activeCountry, search, navigate]);

	const termColumn = table.getColumn("term")!;
	const rankColumn = table.getColumn("rank")!;
	const activeFilter = (rankColumn.getFilterValue() as RankFilter) ?? "all";
	const { pageIndex, pageSize } = table.state.pagination;
	const filteredCount = table.getFilteredRowModel().rows.length;
	const from = filteredCount === 0 ? 0 : pageIndex * pageSize + 1;
	const to = Math.min((pageIndex + 1) * pageSize, filteredCount);

	const counts = Object.fromEntries(
		BUCKETS.map(({ key }) => [
			key,
			data.filter((r) => RANK_FILTERS[key].test(Number(r.rank) || 0)).length,
		]),
	) as Record<Exclude<RankFilter, "all">, number>;
	const total = data.length;

	// Movers: biggest rank changes vs a week ago (falls back to first rank).
	const rankLabel = (r: number | null | undefined) =>
		r == null ? "–" : r === 0 ? "200+" : `#${r}`;
	const movers: Mover[] = data.flatMap((row) => {
		const c = change(row);
		if (!c) return [];
		const prev = row.rank_7d ?? row.rank_first;
		return [
			{
				row,
				delta: c.delta,
				label: `${rankLabel(prev as number)} → ${rankLabel(row.rank as number)}`,
			},
		];
	});
	const byMagnitude = (a: Mover, b: Mover) =>
		Math.abs(b.delta ?? 0) - Math.abs(a.delta ?? 0) ||
		(Number(a.row.rank) || 999) - (Number(b.row.rank) || 999);
	const gainers = movers
		.filter((m) => (m.delta ?? 0) > 0 || (m.delta == null && m.row.rank))
		.sort(byMagnitude)
		.slice(0, 5);
	const losers = movers
		.filter((m) => (m.delta ?? 0) < 0 || (m.delta == null && !m.row.rank))
		.sort(byMagnitude)
		.slice(0, 5);
	const appName = data[0]?.app_name;
	const lastChecked = data
		.map((r) => (r.checked_at ? String(r.checked_at) : ""))
		.sort()
		.at(-1);
	const setFilter = (v: RankFilter) =>
		rankColumn.setFilterValue(v === "all" ? undefined : v);
	const setCountry = (v: string) => {
		table.setPageIndex(0);
		navigate({
			search: (prev) => ({
				...prev,
				country: v === ALL_COUNTRIES ? undefined : v,
				page: undefined,
			}),
			replace: true,
		});
	};

	return (
		<main className="page-wrap pb-16 pt-10">
			<div className="mb-8">
				<h1 className="font-display text-4xl font-bold tracking-tight sm:text-5xl">
					{appName ? `${appName}, ranked` : "Keyword standings"}
				</h1>
				<p className="mt-2 max-w-prose text-muted-foreground">
					Where your app lands in the App Store for {total} tracked keywords
					{activeCountry
						? ` in ${activeCountry.toUpperCase()}`
						: countries.length > 1
							? ` across ${countries.length} countries`
							: ""}
					{lastChecked
						? `, last checked ${new Date(lastChecked.replace(" ", "T")).toLocaleDateString(undefined, { month: "short", day: "numeric" })}`
						: ""}
					.
				</p>
			</div>

			<section aria-label="Rank distribution" className="card mb-6 p-5">
				<div
					className="flex h-3 w-full gap-1 overflow-hidden rounded-full"
					role="img"
					aria-label={BUCKETS.map(
						({ key }) => `${RANK_FILTERS[key].label}: ${counts[key]}`,
					).join(", ")}
				>
					{BUCKETS.map(({ key, bar }, i) =>
						counts[key] ? (
							<div
								key={key}
								className={cn("strip-seg rounded-full", bar)}
								style={{
									flexGrow: counts[key],
									animationDelay: `${i * 90}ms`,
								}}
							/>
						) : null,
					)}
				</div>
				<div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4">
					{BUCKETS.map(({ key, bar }) => {
						const on = activeFilter === key;
						return (
							<button
								key={key}
								type="button"
								aria-pressed={on}
								onClick={() => setFilter(on ? "all" : key)}
								className={cn(
									"flex items-center gap-3 rounded-xl border px-3 py-2.5 text-left transition-colors",
									on
										? "border-primary bg-accent"
										: "border-transparent hover:bg-muted",
								)}
							>
								<span className={cn("size-3 shrink-0 rounded-full", bar)} />
								<span>
									<span className="font-display tnum block text-2xl font-bold leading-none">
										{counts[key]}
									</span>
									<span className="text-sm text-muted-foreground">
										{RANK_FILTERS[key].label}
									</span>
								</span>
							</button>
						);
					})}
				</div>
			</section>

			<section
				aria-label="Biggest movers"
				className="mb-6 grid gap-4 md:grid-cols-2"
			>
				<MoversList
					title="Biggest gainers"
					tone="gain"
					items={gainers}
					empty="No keywords moved up this week."
					showCountry={showCountry}
				/>
				<MoversList
					title="Biggest drops"
					tone="loss"
					items={losers}
					empty="No keywords moved down this week."
					showCountry={showCountry}
				/>
			</section>

			<div className="mb-3 flex flex-wrap items-center gap-3">
				<Input
					type="search"
					aria-label="Search keywords"
					placeholder="Search keywords"
					value={(termColumn.getFilterValue() as string) ?? ""}
					onChange={(e) => termColumn.setFilterValue(e.target.value)}
					className="w-full bg-card sm:w-64"
				/>
				{countries.length > 1 ? (
					<Select
						value={activeCountry ?? ALL_COUNTRIES}
						onValueChange={setCountry}
					>
						<SelectTrigger
							className="w-44 bg-card"
							aria-label="Filter by country"
						>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							<SelectItem value={ALL_COUNTRIES}>
								All countries ({allRanks.length})
							</SelectItem>
							{countries.map(([code, n]) => (
								<SelectItem key={code} value={code}>
									{code.toUpperCase()} ({n})
								</SelectItem>
							))}
						</SelectContent>
					</Select>
				) : null}
				<Select
					value={activeFilter}
					onValueChange={(v) => setFilter(v as RankFilter)}
				>
					<SelectTrigger className="w-44 bg-card" aria-label="Filter by rank">
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{Object.entries(RANK_FILTERS).map(([key, { label }]) => (
							<SelectItem key={key} value={key}>
								{label}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			</div>

			<div className="card overflow-hidden">
				<Table>
					<TableHeader>
						{table.getHeaderGroups().map((headerGroup) => (
							<TableRow key={headerGroup.id} className="hover:bg-transparent">
								{headerGroup.headers.map((header) => {
									const sorted = header.column.getIsSorted();
									const canSort = header.column.getCanSort();
									return (
										<TableHead
											key={header.id}
											aria-sort={
												sorted === "asc"
													? "ascending"
													: sorted === "desc"
														? "descending"
														: undefined
											}
											className={cn(
												header.column.id === "rank_7d" &&
													"hidden sm:table-cell",
											)}
										>
											{header.isPlaceholder ? null : canSort ? (
												<button
													type="button"
													onClick={header.column.getToggleSortingHandler()}
													className={cn(
														"inline-flex items-center gap-1 rounded transition-colors hover:text-foreground",
														sorted && "text-foreground",
													)}
												>
													<table.FlexRender header={header} />
													{sorted === "asc" ? (
														<ArrowUp className="size-3" />
													) : sorted === "desc" ? (
														<ArrowDown className="size-3" />
													) : (
														<ArrowUpDown className="size-3 opacity-40" />
													)}
												</button>
											) : (
												<table.FlexRender header={header} />
											)}
										</TableHead>
									);
								})}
							</TableRow>
						))}
					</TableHeader>
					<TableBody>
						{table.getRowModel().rows.map((row) => (
							<TableRow key={row.id}>
								{row.getAllCells().map((cell) => (
									<TableCell
										key={cell.id}
										className={cn(
											(cell.column.id === "rank_7d" ||
												cell.column.id === "trend") &&
												"hidden sm:table-cell",
										)}
									>
										<table.FlexRender cell={cell} />
									</TableCell>
								))}
							</TableRow>
						))}
						{table.getRowModel().rows.length === 0 && (
							<TableRow className="hover:bg-transparent">
								<TableCell
									colSpan={columns.length}
									className="py-12 text-center text-muted-foreground"
								>
									No keywords match. Try a different search or clear the
									filters.
								</TableCell>
							</TableRow>
						)}
					</TableBody>
				</Table>
			</div>

			<div className="mt-4 flex flex-wrap items-center justify-between gap-3 text-sm text-muted-foreground">
				<div className="tnum">
					{from} to {to} of {filteredCount}
				</div>
				<div className="flex items-center gap-2">
					<Select
						value={String(pageSize)}
						onValueChange={(v) => table.setPageSize(Number(v))}
					>
						<SelectTrigger
							size="sm"
							className="w-32 bg-card"
							aria-label="Rows per page"
						>
							<SelectValue />
						</SelectTrigger>
						<SelectContent>
							{PAGE_SIZES.map((size) => (
								<SelectItem key={size} value={String(size)}>
									{size} per page
								</SelectItem>
							))}
						</SelectContent>
					</Select>
					<Button
						variant="outline"
						size="sm"
						className="bg-card"
						onClick={() => table.previousPage()}
						disabled={!table.getCanPreviousPage()}
					>
						Previous
					</Button>
					<span className="tnum">
						Page {pageIndex + 1} of {Math.max(table.getPageCount(), 1)}
					</span>
					<Button
						variant="outline"
						size="sm"
						className="bg-card"
						onClick={() => table.nextPage()}
						disabled={!table.getCanNextPage()}
					>
						Next
					</Button>
				</div>
			</div>
		</main>
	);
}
