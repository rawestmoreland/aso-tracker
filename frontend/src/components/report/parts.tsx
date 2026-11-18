import { cn } from "cn";
import {
	altitude,
	type Chip as ChipData,
	type ChipKind,
	type Tier,
	tierOf,
} from "#/lib/report";

const CHIP_STYLES: Record<ChipKind, string> = {
	up: "bg-gain/15 text-gain",
	down: "bg-loss/15 text-loss",
	out: "bg-loss/15 text-loss",
	new: "bg-gold text-gold-ink",
	flat: "bg-muted text-muted-foreground",
	none: "bg-muted text-muted-foreground",
};

export function Chip({
	chip,
	className,
}: {
	chip: ChipData;
	className?: string;
}) {
	return (
		<span
			className={cn(
				"inline-block whitespace-nowrap rounded-full px-2.5 py-0.5 text-xs font-bold",
				CHIP_STYLES[chip.kind],
				className,
			)}
		>
			{chip.text}
		</span>
	);
}

export const TIERS: Record<
	Tier,
	{ label: string; bar: string; badge: string; dot: string }
> = {
	summit: {
		label: "Summit",
		bar: "bg-gold",
		badge: "bg-gold text-gold-ink",
		dot: "bg-gold",
	},
	ridge: {
		label: "Ridge",
		bar: "bg-gain",
		badge: "bg-gain/15 text-gain",
		dot: "bg-gain",
	},
	foothills: {
		label: "Foothills",
		bar: "bg-primary",
		badge: "bg-primary/15 text-primary",
		dot: "bg-primary",
	},
	base: {
		label: "Base camp",
		bar: "bg-muted-foreground/40",
		badge: "bg-muted text-muted-foreground",
		dot: "bg-muted-foreground/40",
	},
};

export function TierBadge({
	tier,
	className,
}: {
	tier: Tier;
	className?: string;
}) {
	return (
		<span
			className={cn(
				"inline-block rounded-full px-2 py-0.5 text-[11px] font-bold uppercase tracking-wider",
				TIERS[tier].badge,
				className,
			)}
		>
			{TIERS[tier].label}
		</span>
	);
}

const rankLabel = (r: number) => (r > 0 ? `#${r}` : "unranked");

/**
 * A mini bar chart of daily rank. Taller is better ("altitude"), and each bar
 * takes its tier's colour so you can see a keyword cross into the top 10.
 */
export function Spark({
	ranks,
	className,
}: {
	ranks: number[];
	className?: string;
}) {
	const known = ranks.filter((r) => r >= 0);
	const summary =
		known.length > 1
			? `Rank over the last ${ranks.length} days: ${rankLabel(known[0])} to ${rankLabel(known[known.length - 1])}`
			: "Not enough history yet";
	return (
		<div
			role="img"
			aria-label={summary}
			className={cn("flex h-8 items-end gap-0.5", className)}
		>
			{ranks.map((r, i) => {
				// Heights are percentages of the container; never fully flat.
				const height = r < 0 ? 4 : r === 0 ? 10 : 10 + altitude(r) * 0.9;
				const colour =
					r < 0
						? "bg-border"
						: r === 0
							? "bg-muted-foreground/20"
							: TIERS[tierOf(r)].bar;
				return (
					<div
						// biome-ignore lint/suspicious/noArrayIndexKey: a fixed-length series; position is the identity
						key={i}
						className={cn("w-2 rounded-t-[3px]", colour)}
						style={{ height: `${height}%` }}
					/>
				);
			})}
		</div>
	);
}

export function Kicker({ children }: { children: React.ReactNode }) {
	return (
		<p className="text-xs font-bold uppercase tracking-[0.18em] text-muted-foreground">
			{children}
		</p>
	);
}
