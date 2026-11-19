import { Link } from "@tanstack/react-router";
import ThemeToggle from "./ThemeToggle";

function Mark() {
	// Three bars of different heights: a podium, and a rank chart at once.
	return (
		<svg viewBox="0 0 24 24" width="26" height="26" aria-hidden="true">
			<rect
				x="2"
				y="11"
				width="6"
				height="11"
				rx="1.5"
				fill="var(--muted-foreground)"
				opacity="0.55"
			/>
			<rect x="9" y="3" width="6" height="19" rx="1.5" fill="var(--primary)" />
			<rect x="16" y="14" width="6" height="8" rx="1.5" fill="var(--gold)" />
		</svg>
	);
}

export default function Header() {
	return (
		<header className="sticky top-0 z-40 border-b border-border bg-background/80 backdrop-blur-lg">
			<div className="page-wrap flex items-center justify-between py-3">
				<Link
					to="/"
					className="flex items-center gap-2.5 no-underline"
					aria-label="Standings home"
				>
					<Mark />
					<span className="font-display hidden text-xl font-bold tracking-tight sm:inline">
						Standings
					</span>
				</Link>
				<div className="flex items-center gap-5">
					<nav
						aria-label="Main"
						className="flex items-center gap-3 whitespace-nowrap text-sm font-medium sm:gap-4"
					>
						<Link
							to="/"
							className="text-muted-foreground no-underline transition-colors hover:text-foreground [&.active]:text-foreground"
						>
							Keywords
						</Link>
						<Link
							to="/gaps"
							className="text-muted-foreground no-underline transition-colors hover:text-foreground [&.active]:text-foreground"
						>
							Gaps
						</Link>
						<Link
							to="/reports"
							className="text-muted-foreground no-underline transition-colors hover:text-foreground [&.active]:text-foreground"
						>
							Reports
						</Link>
					</nav>
					<ThemeToggle />
				</div>
			</div>
		</header>
	);
}
