import { createFileRoute, Link } from "@tanstack/react-router";
import { cn } from "cn";
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
import type { LatestRanksRecord } from "#/lib/pocketbase-types";

const TOPS = [3, 10, 20, 50];
const DEFAULT_TOP = 10;
const UNRANKED = 201; // rank 0 means "not in the top 200"
const ANY = "any";

type GapsSearch = { q?: string; top?: number; comp?: string };

export const Route = createFileRoute("/gaps")({
	ssr: false,
	validateSearch: (s: Record<string, unknown>): GapsSearch => {
		const top = Number(s.top);
		return {
			q: typeof s.q === "string" && s.q ? s.q : undefined,
			top: TOPS.includes(top) && top !== DEFAULT_TOP ? top : undefined,
			comp: typeof s.comp === "string" && s.comp ? s.comp : undefined,
		};
	},
	loader: async ({ context }) =>
		context.pb.collection<LatestRanksRecord>("latest_ranks").getFullList(),
	component: Gaps,
});

type Gap = {
	keyword: string;
	term: string;
	country: string;
	mine: number; // 0 = unranked
	rival: { name: string; rank: number };
	gap: number;
};

function rankText(r: number) {
	return r ? `#${r}` : "200+";
}

function Gaps() {
	const rows = Route.useLoaderData();
	const search = Route.useSearch();
	const navigate = Route.useNavigate();
	const top = search.top ?? DEFAULT_TOP;
	const comp = search.comp ?? ANY;
	const q = (search.q ?? "").toLowerCase();
	const set = (patch: Partial<GapsSearch>) =>
		navigate({ search: (prev) => ({ ...prev, ...patch }), replace: true });

	const competitors = new Map<string, string>();
	const byKeyword = new Map<string, LatestRanksRecord[]>();
	for (const r of rows) {
		if (!r.is_mine && r.app)
			competitors.set(String(r.app), r.app_name || String(r.app));
		const k = String(r.keyword);
		byKeyword.set(k, [...(byKeyword.get(k) ?? []), r]);
	}

	const gaps: Gap[] = [];
	for (const [keyword, group] of byKeyword) {
		const mineRow = group.find((r) => r.is_mine);
		if (!mineRow) continue;
		const mine = Number(mineRow.rank) || 0;
		const best = group
			.filter(
				(r) =>
					!r.is_mine &&
					Number(r.rank) > 0 &&
					(comp === ANY || String(r.app) === comp),
			)
			.sort((a, b) => Number(a.rank) - Number(b.rank))[0];
		if (!best) continue;
		const rival = Number(best.rank);
		const effective = mine || UNRANKED;
		if (rival > top || effective <= rival) continue;
		if (q && !mineRow.term.toLowerCase().includes(q)) continue;
		gaps.push({
			keyword,
			term: mineRow.term,
			country: mineRow.country,
			mine,
			rival: { name: best.app_name || String(best.app), rank: rival },
			gap: effective - rival,
		});
	}
	gaps.sort((a, b) => b.gap - a.gap || a.term.localeCompare(b.term));

	return (
		<main className="page-wrap pb-16 pt-10">
			<div className="mb-8">
				<h1 className="font-display text-4xl font-bold tracking-tight sm:text-5xl">
					Competitor gaps
				</h1>
				<p className="mt-2 max-w-prose text-muted-foreground">
					Keywords where a competitor ranks in the top {top} and your app ranks
					lower or not at all. The biggest gaps come first.
				</p>
			</div>

			<div className="mb-3 flex flex-wrap items-center gap-3">
				<Input
					type="search"
					aria-label="Search keywords"
					placeholder="Search keywords"
					value={search.q ?? ""}
					onChange={(e) => set({ q: e.target.value || undefined })}
					className="w-full bg-card sm:w-64"
				/>
				<Select
					value={String(top)}
					onValueChange={(v) =>
						set({ top: Number(v) === DEFAULT_TOP ? undefined : Number(v) })
					}
				>
					<SelectTrigger className="w-44 bg-card" aria-label="Competitor rank">
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						{TOPS.map((t) => (
							<SelectItem key={t} value={String(t)}>
								Rival in top {t}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
				<Select
					value={comp}
					onValueChange={(v) => set({ comp: v === ANY ? undefined : v })}
				>
					<SelectTrigger className="w-52 bg-card" aria-label="Competitor">
						<SelectValue />
					</SelectTrigger>
					<SelectContent>
						<SelectItem value={ANY}>Any competitor</SelectItem>
						{[...competitors].map(([id, name]) => (
							<SelectItem key={id} value={id}>
								{name}
							</SelectItem>
						))}
					</SelectContent>
				</Select>
			</div>

			<div className="card overflow-hidden">
				<Table>
					<TableHeader>
						<TableRow className="hover:bg-transparent">
							<TableHead>Keyword</TableHead>
							<TableHead>You</TableHead>
							<TableHead>Best competitor</TableHead>
							<TableHead className="text-right">Gap</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{gaps.map((g) => (
							<TableRow key={g.keyword}>
								<TableCell>
									<Link
										to="/keywords/$keywordId"
										params={{ keywordId: g.keyword }}
										className="row-link text-base font-semibold hover:text-primary"
									>
										{g.term}
									</Link>
								</TableCell>
								<TableCell
									className={cn(
										"font-display tnum text-lg font-bold",
										!g.mine && "text-muted-foreground",
									)}
								>
									{rankText(g.mine)}
								</TableCell>
								<TableCell>
									<span className="font-display tnum mr-2 text-lg font-bold">
										#{g.rival.rank}
									</span>
									<span className="text-sm text-muted-foreground">
										{g.rival.name}
									</span>
								</TableCell>
								<TableCell className="tnum text-right">
									<span className="rounded-full bg-loss/12 px-2 py-0.5 text-xs font-semibold text-loss">
										{g.mine ? `${g.gap} behind` : "Unranked"}
									</span>
								</TableCell>
							</TableRow>
						))}
						{gaps.length === 0 && (
							<TableRow className="hover:bg-transparent">
								<TableCell
									colSpan={4}
									className="py-12 text-center text-muted-foreground"
								>
									No gaps found. Try widening the competitor rank or clearing
									the search.
								</TableCell>
							</TableRow>
						)}
					</TableBody>
				</Table>
			</div>
			<p className="tnum mt-4 text-sm text-muted-foreground">
				{gaps.length} {gaps.length === 1 ? "gap" : "gaps"}
			</p>
		</main>
	);
}
