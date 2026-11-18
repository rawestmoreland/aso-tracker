import type { KeywordRow } from "#/lib/report";
import { Chip, Spark, TierBadge } from "./parts";

function Country({ code }: { code: string }) {
	return (
		<span className="text-[10px] font-bold tracking-widest text-muted-foreground">
			{code}
		</span>
	);
}

/** A keyword as its own card: rank, change, trend, and Claude's comment. */
export function KeywordCard({ row }: { row: KeywordRow }) {
	return (
		<article className="card p-4 sm:p-5">
			<div className="flex items-start justify-between gap-4">
				<div className="min-w-0">
					<h3 className="text-lg font-semibold leading-snug">
						{row.term} <Country code={row.country} />
					</h3>
					<p className="mt-0.5 text-sm text-muted-foreground">{row.detail}</p>
				</div>
				<div className="shrink-0 text-right">
					<div className="font-display text-3xl font-bold leading-none tnum">
						{row.rank_text}
					</div>
					<div className="mt-1.5">
						<Chip chip={row.change} />
					</div>
				</div>
			</div>
			<div className="mt-3 flex items-center justify-between gap-3">
				<Spark ranks={row.daily_ranks} />
				<TierBadge tier={row.tier} />
			</div>
			{row.comment ? (
				<p className="mt-3 text-[15px] leading-relaxed">{row.comment}</p>
			) : null}
		</article>
	);
}

/** A one-line keyword reference, used inside stories, callouts and quests. */
export function KeywordStrip({ row }: { row: KeywordRow }) {
	return (
		<div className="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2 rounded-xl bg-muted/70 px-3 py-2">
			<span className="text-sm font-semibold">
				{row.term} <Country code={row.country} />
			</span>
			<span className="flex items-baseline gap-1.5">
				<span className="text-xs text-muted-foreground">you</span>
				<span className="font-display text-xl font-bold leading-none tnum">
					{row.rank_text}
				</span>
			</span>
			<Chip chip={row.change} />
			<Spark ranks={row.daily_ranks} className="ml-auto" />
		</div>
	);
}
