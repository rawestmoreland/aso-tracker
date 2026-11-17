import { createFileRoute, Link } from "@tanstack/react-router";
import { cn } from "cn";
import { ArrowLeft } from "lucide-react";
import {
	CartesianGrid,
	Line,
	LineChart,
	ResponsiveContainer,
	Tooltip,
	XAxis,
	YAxis,
} from "recharts";
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

const RANGES = [
	{ key: "7", label: "7 days", days: 7 },
	{ key: "30", label: "30 days", days: 30 },
	{ key: "90", label: "90 days", days: 90 },
	{ key: "all", label: "All time", days: null },
] as const;
type RangeKey = (typeof RANGES)[number]["key"];

export const Route = createFileRoute("/keywords/$keywordId")({
	ssr: false,
	// "all" is the default, so it stays out of the URL.
	validateSearch: (s: Record<string, unknown>): { range?: RangeKey } => {
		const hit = RANGES.find((r) => r.key === String(s.range));
		return { range: hit && hit.key !== "all" ? hit.key : undefined };
	},
	loader: async ({ context, params }) => {
		const { pb } = context;
		const [history, apps] = await Promise.all([
			pb.collection<RankHistoryRecord>("rank_history").getFullList({
				filter: pb.filter("keyword = {:id}", { id: params.keywordId }),
				sort: "checked_at",
			}),
			// latest_ranks has one row per tracked app for this keyword, which gives
			// us the app names and is_mine without needing access to tracked_apps.
			pb.collection<LatestRanksRecord>("latest_ranks").getFullList({
				filter: pb.filter("keyword = {:id}", { id: params.keywordId }),
			}),
		]);
		return { history, apps };
	},
	component: KeywordDetails,
});

const COLORS = [1, 2, 3, 4, 5, 6].map((n) => `var(--series-${n})`);
const MAX_RANK = 200; // iTunes search returns at most 200 results

const day = (iso: string) => iso.slice(0, 10);

function KeywordDetails() {
	const { history: fullHistory, apps } = Route.useLoaderData();
	const range = Route.useSearch().range ?? "all";
	const days = RANGES.find((r) => r.key === range)?.days ?? null;
	const cutoff =
		days == null
			? ""
			: new Date(Date.now() - days * 86400_000).toISOString().slice(0, 10);
	const history = cutoff
		? fullHistory.filter((h) => day(h.checked_at) >= cutoff)
		: fullHistory;

	const appList = [...apps]
		.map((a) => ({
			id: String(a.app),
			name: a.app_name || String(a.app),
			isMine: !!a.is_mine,
		}))
		// "mine" first so it gets the primary colour
		.sort((a, b) => Number(b.isMine) - Number(a.isMine));
	const appName = new Map(appList.map((a) => [a.id, a.name]));

	// One point per app per day (last check of the day wins). Rank 0 means
	// "not in the top 200", so it becomes null -> a gap in the line.
	const byDay = new Map<string, Record<string, number | null | string>>();
	for (const h of history) {
		const d = day(h.checked_at);
		const row = byDay.get(d) ?? { date: d };
		row[h.app] = h.rank ? h.rank : null;
		byDay.set(d, row);
	}
	const chartData = [...byDay.values()].sort((a, b) =>
		String(a.date).localeCompare(String(b.date)),
	);

	const seen = history.map((h) => h.rank).filter((r): r is number => !!r);
	const worst = seen.length ? Math.min(MAX_RANK, Math.max(...seen)) : 10;

	const term = apps[0]?.term;
	const country = apps[0]?.country;
	const checks = [...history].reverse().slice(0, 100);

	const latest = (appId: string) => {
		for (let i = fullHistory.length - 1; i >= 0; i--)
			if (fullHistory[i].app === appId) return fullHistory[i].rank || 0;
		return null;
	};
	const axis = { fill: "var(--muted-foreground)", fontSize: 12 };

	return (
		<main className="page-wrap pb-16 pt-8">
			<Link
				to="/"
				className="inline-flex items-center gap-1.5 text-sm text-muted-foreground no-underline transition-colors hover:text-foreground"
			>
				<ArrowLeft className="size-4" />
				All keywords
			</Link>
			<div className="mb-6 mt-3 flex flex-wrap items-center gap-3">
				<h1 className="font-display text-4xl font-bold tracking-tight sm:text-5xl">
					{term ?? "Keyword"}
				</h1>
				{country ? (
					<span className="rounded-full border border-border bg-card px-2.5 py-0.5 text-xs font-semibold uppercase text-muted-foreground">
						{country}
					</span>
				) : null}
			</div>

			<ul className="mb-6 flex flex-wrap gap-3">
				{appList.map((a, i) => {
					const r = latest(a.id);
					return (
						<li
							key={a.id}
							className={cn(
								"card flex items-center gap-3 py-2.5 pl-3 pr-5",
								a.isMine && "border-primary/50",
							)}
						>
							<span
								className="size-3 shrink-0 rounded-full"
								style={{ background: COLORS[i % COLORS.length] }}
							/>
							<span className="min-w-0">
								<span className="block max-w-48 truncate text-sm font-semibold">
									{a.name}
								</span>
								<span className="text-xs text-muted-foreground">
									{a.isMine ? "Your app" : "Competitor"}
								</span>
							</span>
							<span className="font-display tnum ml-2 text-2xl font-bold">
								{r == null ? "–" : r ? `#${r}` : "200+"}
							</span>
						</li>
					);
				})}
			</ul>

			<nav
				aria-label="Date range"
				className="mb-3 inline-flex gap-1 rounded-xl border border-border bg-card p-1"
			>
				{RANGES.map((r) => (
					<Link
						key={r.key}
						to="."
						search={{ range: r.key === "all" ? undefined : r.key }}
						replace
						aria-current={range === r.key ? "true" : undefined}
						className={cn(
							"rounded-lg px-3 py-1 text-sm font-medium no-underline transition-colors",
							range === r.key
								? "bg-accent text-accent-foreground"
								: "text-muted-foreground hover:text-foreground",
						)}
					>
						{r.label}
					</Link>
				))}
			</nav>

			<div className="card h-96 w-full p-4 pb-2">
				{chartData.length === 0 ? (
					<p className="grid h-full place-items-center text-muted-foreground">
						No checks in this range.
					</p>
				) : (
					<ResponsiveContainer width="100%" height="100%">
						<LineChart data={chartData} margin={{ top: 8, right: 16, left: 0 }}>
							<CartesianGrid vertical={false} stroke="var(--border)" />
							<XAxis
								dataKey="date"
								tick={axis}
								axisLine={{ stroke: "var(--border)" }}
								tickLine={false}
								tickMargin={8}
							/>
							<YAxis
								reversed
								allowDecimals={false}
								domain={[1, worst]}
								tick={axis}
								axisLine={false}
								tickLine={false}
								width={44}
								tickFormatter={(v) => `#${v}`}
							/>
							<Tooltip
								cursor={{ stroke: "var(--border)" }}
								contentStyle={{
									background: "var(--popover)",
									border: "1px solid var(--border)",
									borderRadius: 12,
									color: "var(--popover-foreground)",
									boxShadow: "var(--shadow-card)",
								}}
								formatter={(v, name) => [
									v == null ? "200+" : `#${v}`,
									appName.get(String(name)) ?? name,
								]}
							/>
							{appList.map((a, i) => (
								<Line
									key={a.id}
									dataKey={a.id}
									name={a.id}
									stroke={COLORS[i % COLORS.length]}
									strokeWidth={a.isMine ? 3.5 : 1.75}
									strokeLinecap="round"
									connectNulls={false}
									dot={{
										r: a.isMine ? 4 : 2.5,
										strokeWidth: 2,
										fill: "var(--card)",
									}}
									activeDot={{ r: 6 }}
									isAnimationActive={false}
								/>
							))}
							{/* TODO: <ReferenceLine x='YYYY-MM-DD' /> for title/subtitle changes */}
						</LineChart>
					</ResponsiveContainer>
				)}
			</div>

			<h2 className="font-display mb-3 mt-10 text-2xl font-bold tracking-tight">
				Recent checks
			</h2>
			<div className="card overflow-hidden">
				<Table>
					<TableHeader>
						<TableRow className="hover:bg-transparent">
							<TableHead>Checked</TableHead>
							<TableHead>App</TableHead>
							<TableHead>Rank</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{checks.map((c) => (
							<TableRow key={c.id}>
								<TableCell className="tnum text-muted-foreground">
									{new Date(c.checked_at.replace(" ", "T")).toLocaleString()}
								</TableCell>
								<TableCell className="font-medium">
									{appName.get(c.app) ?? c.app}
								</TableCell>
								<TableCell className="font-display tnum text-lg font-bold">
									{c.rank ? (
										`#${c.rank}`
									) : (
										<span className="text-muted-foreground">200+</span>
									)}
								</TableCell>
							</TableRow>
						))}
					</TableBody>
				</Table>
			</div>
		</main>
	);
}
