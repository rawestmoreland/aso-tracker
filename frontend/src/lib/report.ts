// Shape of the resolved report the backend stores in `reports.view` and exposes
// through the read-only `public_reports` collection. Keep in sync with the
// `view` structs in backend/internal/report/render.go. The backend bumps
// `version` when the shape changes incompatibly.

export const SUPPORTED_VIEW_VERSION = 1;

export type ChipKind = "up" | "down" | "flat" | "new" | "out" | "none";
/** summit = top 3, ridge = top 10, foothills = top 30, base = everything else. */
export type Tier = "summit" | "ridge" | "foothills" | "base";
export type MoodKey = "summit" | "climbing" | "steady" | "slipping" | "storm";

export type Chip = { text: string; kind: ChipKind };

export type KeywordRow = {
	term: string;
	country: string;
	/** "#12", or "200+" when not in the top 200. */
	rank_text: string;
	/** 0 means not in the top 200. */
	rank: number;
	tier: Tier;
	change: Chip;
	detail: string;
	/** Oldest first. 0 = not in the top 200, -1 = not tracked yet. */
	daily_ranks: number[];
	comment?: string;
};

/** One storefront's own visibility score. Present only when keywords span 2+ countries. */
export type CountryRow = {
	/** Upper-case storefront code, e.g. "GB". */
	country: string;
	keywords: number;
	/** 0-100, same scale as the headline Visibility tile. */
	visibility: number;
	/** Change in visibility over the report window. */
	change: Chip;
	top10: number;
	top3: number;
	/** Daily visibility, oldest first. */
	daily_visibility: number[];
};

export type Tile = { label: string; value: string; sub: string; delta: Chip };

export type Highlight = {
	kind: "win" | "loss" | "surprise" | "milestone";
	emoji: string;
	title: string;
	body: string;
	keyword?: KeywordRow;
};

export type Section = { title: string; intro: string; rows: KeywordRow[] };

export type Callout = { app: string; note: string; keyword?: KeywordRow };

export type Quest = {
	n: number;
	title: string;
	why: string;
	how: string;
	effort: string;
	impact: string;
	effort_key: "quick" | "medium" | "big";
	impact_key: "low" | "medium" | "high";
	keywords: KeywordRow[];
};

export type ReportView = {
	version: number;
	subject: string;
	preheader: string;
	mood: { key: MoodKey; label: string; emoji: string };
	week: string;
	app_name: string;
	icon_url: string;
	headline: string;
	intro: string;
	scoreboard_note: string;
	/** Daily visibility score, oldest first. */
	hero_series: number[];
	tiles: Tile[];
	/** Absent on reports stored before per-country scores existed. */
	countries?: CountryRow[];
	highlights: Highlight[];
	climbers: Section;
	fallers: Section;
	rivals_title: string;
	rivals_body: string;
	callouts: Callout[];
	quests: Quest[];
	sign_off: string;
	footer: string;
};

/** True when `v` is a report this build of the page knows how to render. */
export function isReportView(v: unknown): v is ReportView {
	return (
		typeof v === "object" &&
		v !== null &&
		(v as { version?: unknown }).version === SUPPORTED_VIEW_VERSION
	);
}

/** 0-100 altitude of a rank: #1 is 100, outside the top 100 is 0. */
export function altitude(rank: number): number {
	return rank >= 1 && rank <= 100 ? 101 - rank : 0;
}

export function tierOf(rank: number): Tier {
	if (rank >= 1 && rank <= 3) return "summit";
	if (rank >= 1 && rank <= 10) return "ridge";
	if (rank >= 1 && rank <= 30) return "foothills";
	return "base";
}
