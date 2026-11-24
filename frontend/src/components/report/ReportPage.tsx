import { cn } from "cn";
import type {
	CountryRow,
	Highlight,
	MoodKey,
	Quest,
	ReportView,
	Section,
} from "#/lib/report";
import { KeywordCard, KeywordStrip } from "./KeywordRow";
import { Chip, Kicker, TIERS } from "./parts";

// Each mood gets its own sky. Literal class names so Tailwind can see them.
const MOODS: Record<MoodKey, string> = {
	summit: "bg-linear-to-br from-amber-500 to-orange-600",
	climbing: "bg-linear-to-br from-teal-700 to-emerald-500",
	steady: "bg-linear-to-br from-sky-800 to-sky-500",
	slipping: "bg-linear-to-br from-amber-700 to-orange-700",
	storm: "bg-linear-to-br from-slate-800 to-slate-600",
};

const HIGHLIGHT_BORDER: Record<Highlight["kind"], string> = {
	win: "border-l-gain",
	loss: "border-l-loss",
	surprise: "border-l-primary",
	milestone: "border-l-gold",
};

const IMPACT_STYLES: Record<Quest["impact_key"], string> = {
	high: "bg-gain/15 text-gain",
	medium: "bg-primary/15 text-primary",
	low: "bg-muted text-muted-foreground",
};

function SectionHead({
	kicker,
	title,
	intro,
}: {
	kicker: string;
	title: string;
	intro?: string;
}) {
	return (
		<div className="mb-5">
			<Kicker>{kicker}</Kicker>
			<h2 className="mt-1 font-display text-3xl font-bold tracking-tight">
				{title}
			</h2>
			{intro ? (
				<p className="mt-2 max-w-prose text-muted-foreground">{intro}</p>
			) : null}
		</div>
	);
}

/** The daily visibility score as a skyline, scaled to its own range. */
function Skyline({ series }: { series: number[] }) {
	const lo = Math.min(...series);
	const hi = Math.max(...series);
	return (
		<div
			role="img"
			aria-label={`Visibility score over the last ${series.length} days`}
			className="mt-8 flex h-24 items-end gap-1.5 sm:gap-2"
		>
			{series.map((v, i) => {
				const frac = hi > lo ? (v - lo) / (hi - lo) : 0.5;
				return (
					<div
						// biome-ignore lint/suspicious/noArrayIndexKey: a fixed-length series; position is the identity
						key={i}
						className={cn(
							"flex-1 rounded-t-lg bg-white",
							i === series.length - 1 ? "opacity-100" : "opacity-40",
						)}
						style={{ height: `${14 + frac * 86}%` }}
					/>
				);
			})}
		</div>
	);
}

function Hero({ view }: { view: ReportView }) {
	return (
		<header
			className={cn(
				"overflow-hidden rounded-3xl p-6 text-white shadow-[var(--shadow-card)] sm:p-10",
				MOODS[view.mood.key] ?? MOODS.steady,
			)}
		>
			<div className="flex items-center gap-3">
				{view.icon_url ? (
					<img
						src={view.icon_url}
						alt=""
						width={40}
						height={40}
						referrerPolicy="no-referrer"
						className="size-10 rounded-[10px] bg-white/20"
					/>
				) : null}
				<div className="leading-tight">
					<p className="font-display text-lg font-bold">The Weekly Climb</p>
					<p className="text-sm text-white/80">
						{view.app_name} · {view.week}
					</p>
				</div>
			</div>
			<p className="mt-8 inline-block rounded-full bg-white/20 px-3 py-1.5 text-xs font-bold uppercase tracking-[0.18em]">
				<span aria-hidden="true">{view.mood.emoji}</span> {view.mood.label}
			</p>
			<h1 className="mt-4 font-display text-4xl font-bold leading-[1.05] tracking-tight sm:text-5xl">
				{view.headline}
			</h1>
			<Skyline series={view.hero_series} />
			<p className="mt-2 text-[11px] font-bold uppercase tracking-[0.18em] text-white/80">
				Visibility · last {view.hero_series.length} days
			</p>
		</header>
	);
}

function Scoreboard({ view }: { view: ReportView }) {
	return (
		<section aria-label="Scoreboard">
			<div className="grid gap-3 sm:grid-cols-3">
				{view.tiles.map((t) => (
					<div key={t.label} className="card p-5">
						<Kicker>{t.label}</Kicker>
						<p className="mt-2 font-display text-5xl font-bold leading-none tnum">
							{t.value}
						</p>
						<div className="mt-3">
							<Chip chip={t.delta} />
						</div>
						<p className="mt-2 text-sm text-muted-foreground">{t.sub}</p>
					</div>
				))}
			</div>
			{view.scoreboard_note ? (
				<p className="mt-4 italic text-muted-foreground">
					{view.scoreboard_note}
				</p>
			) : null}
			<ul className="mt-3 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
				{(
					[
						["summit", "summit, top 3"],
						["ridge", "ridge, top 10"],
						["foothills", "foothills, top 30"],
						["base", "base camp"],
					] as const
				).map(([tier, label]) => (
					<li key={tier} className="flex items-center gap-1.5">
						<span
							className={cn("size-2.5 rounded-[3px]", TIERS[tier].dot)}
							aria-hidden="true"
						/>
						{label}
					</li>
				))}
			</ul>
		</section>
	);
}

/**
 * Each storefront's own visibility score. The headline tile blends every
 * country, so a weak new market can't hide in (or drag down) a strong one.
 */
function Countries({ rows }: { rows: CountryRow[] }) {
	if (rows.length < 2) return null;
	return (
		<section aria-label="Visibility by country">
			<Kicker>By country</Kicker>
			<ul className="card mt-3 divide-y">
				{rows.map((c) => (
					<li
						key={c.country}
						className="grid grid-cols-[4.5rem_1fr_auto] items-center gap-4 p-4 sm:grid-cols-[6rem_1fr_auto] sm:p-5"
					>
						<div>
							<p className="text-base font-extrabold tracking-wider">
								{c.country}
							</p>
							<p className="text-xs text-muted-foreground">
								{c.keywords} keywords
							</p>
						</div>
						<div>
							<p className="font-display text-3xl font-bold leading-none tnum">
								{c.visibility.toFixed(1)}
							</p>
							<div
								role="img"
								aria-label={`Visibility ${c.visibility.toFixed(1)} out of 100`}
								className="mt-2 h-1.5 overflow-hidden rounded-full bg-muted"
							>
								<div
									className="h-full rounded-full bg-primary"
									style={{
										width: `${Math.min(100, Math.max(2, c.visibility))}%`,
									}}
								/>
							</div>
						</div>
						<div className="text-right">
							<Chip chip={c.change} />
							<p className="mt-1 text-xs text-muted-foreground">
								{c.top10} in top 10
							</p>
						</div>
					</li>
				))}
			</ul>
			<p className="mt-2 text-xs text-muted-foreground">
				The headline score blends every country. Each row is that storefront's
				own visibility score, 0–100.
			</p>
		</section>
	);
}

function Highlights({ items }: { items: Highlight[] }) {
	if (items.length === 0) return null;
	return (
		<section>
			<Kicker>The week in stories</Kicker>
			<div className="mt-4 space-y-3">
				{items.map((h) => (
					<article
						key={h.title}
						className={cn("card border-l-[5px] p-5", HIGHLIGHT_BORDER[h.kind])}
					>
						<h3 className="font-display text-xl font-bold leading-snug">
							<span aria-hidden="true">{h.emoji}</span> {h.title}
						</h3>
						<p className="mt-1.5 leading-relaxed">{h.body}</p>
						{h.keyword ? <KeywordStrip row={h.keyword} /> : null}
					</article>
				))}
			</div>
		</section>
	);
}

function Movers({ kicker, section }: { kicker: string; section: Section }) {
	if (section.rows.length === 0) return null;
	return (
		<section>
			<SectionHead
				kicker={kicker}
				title={section.title}
				intro={section.intro}
			/>
			<div className="space-y-3">
				{section.rows.map((row) => (
					<KeywordCard key={`${row.term}-${row.country}`} row={row} />
				))}
			</div>
		</section>
	);
}

function Rivals({ view }: { view: ReportView }) {
	if (!view.rivals_title && view.callouts.length === 0) return null;
	return (
		<section>
			<SectionHead
				kicker="Over the next ridge"
				title={view.rivals_title || "Competitor corner"}
				intro={view.rivals_body}
			/>
			<div className="space-y-4">
				{view.callouts.map((c) => (
					<div
						key={`${c.app}-${c.note}`}
						className="border-l-[3px] border-l-primary pl-4"
					>
						<p className="text-xs font-extrabold uppercase tracking-[0.14em] text-primary">
							{c.app}
						</p>
						<p className="mt-0.5 leading-relaxed">{c.note}</p>
						{c.keyword ? <KeywordStrip row={c.keyword} /> : null}
					</div>
				))}
			</div>
		</section>
	);
}

function Quests({ quests }: { quests: Quest[] }) {
	if (quests.length === 0) return null;
	return (
		<section className="rounded-3xl border border-border bg-secondary/60 p-5 sm:p-8">
			<SectionHead
				kicker="Your move"
				title="This week's quests"
				intro="Ordered by payoff for effort. Do the first one and you've already had a good Monday."
			/>
			<ol className="space-y-3">
				{quests.map((q) => (
					<li key={q.n} className="card flex gap-4 p-4 sm:p-5">
						<span
							aria-hidden="true"
							className="flex size-9 shrink-0 items-center justify-center rounded-full bg-gold font-display text-lg font-bold text-gold-ink"
						>
							{q.n}
						</span>
						<div className="min-w-0">
							<h3 className="font-display text-xl font-bold leading-snug">
								{q.title}
							</h3>
							<p className="mt-2 flex flex-wrap gap-2">
								<span
									className={cn(
										"rounded-full px-2.5 py-0.5 text-xs font-bold",
										IMPACT_STYLES[q.impact_key],
									)}
								>
									{q.impact}
								</span>
								<span className="rounded-full bg-muted px-2.5 py-0.5 text-xs font-bold text-muted-foreground">
									{q.effort}
								</span>
							</p>
							<p className="mt-3 leading-relaxed">
								<strong className="text-primary">Why:</strong> {q.why}
							</p>
							<p className="mt-2 leading-relaxed">
								<strong className="text-primary">How:</strong> {q.how}
							</p>
							{q.keywords.map((k) => (
								<KeywordStrip key={`${k.term}-${k.country}`} row={k} />
							))}
						</div>
					</li>
				))}
			</ol>
		</section>
	);
}

export function ReportPage({ view }: { view: ReportView }) {
	return (
		<main className="page-wrap pb-20 pt-8">
			<div className="mx-auto max-w-3xl space-y-12">
				<Hero view={view} />
				<p className="font-display text-xl leading-9 sm:text-2xl sm:leading-10">
					{view.intro}
				</p>
				<Scoreboard view={view} />
				<Countries rows={view.countries ?? []} />
				<Highlights items={view.highlights} />
				<Movers kicker="Uphill" section={view.climbers} />
				<Movers kicker="Downhill" section={view.fallers} />
				<Rivals view={view} />
				<Quests quests={view.quests} />
				<footer className="pt-4 text-center">
					<p aria-hidden="true" className="text-3xl tracking-[0.4em]">
						⛰️ 🏔️ ⛰️
					</p>
					<p className="mx-auto mt-4 max-w-xl font-display text-xl italic leading-8">
						{view.sign_off}
					</p>
					<p className="mt-6 text-xs text-muted-foreground">{view.footer}</p>
				</footer>
			</div>
		</main>
	);
}
