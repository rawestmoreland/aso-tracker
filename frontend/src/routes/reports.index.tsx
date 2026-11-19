import { createFileRoute, Link, useRouter } from "@tanstack/react-router";
import { cn } from "cn";
import { LogOut } from "lucide-react";
import { type FormEvent, useState } from "react";
import { Button } from "#/components/ui/button";
import { Input } from "#/components/ui/input";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "#/components/ui/table";
import type { ReportsRecord } from "#/lib/pocketbase-types";

/**
 * Every generated report, newest first.
 *
 * Unlike the rest of the dashboard this page isn't public. A report's slug is
 * its access credential (`/reports/$slug` is readable by anyone holding it), so
 * listing slugs has to stay behind the same wall as the `reports` collection:
 * superuser only. The page therefore asks for a PocketBase superuser sign-in
 * (kept in the SDK's localStorage auth store) instead of reading
 * `public_reports`, which by design can't be listed.
 */

// The heavy columns (`html`, `content`, `view`) are never needed for a list.
const LIST_FIELDS =
	"id,slug,subject,headline,model,recipients,sent_at,created,usage";

type LoaderData =
	| { status: "signed-out"; reports: [] }
	| { status: "ready"; reports: ReportsRecord[]; email: string };

export const Route = createFileRoute("/reports/")({
	ssr: false,
	loader: async ({ context }): Promise<LoaderData> => {
		const { pb } = context;
		const isSuperuser =
			pb.authStore.isValid &&
			pb.authStore.record?.collectionName === "_superusers";
		if (!isSuperuser) return { status: "signed-out", reports: [] };
		try {
			const reports = await pb
				.collection<ReportsRecord>("reports")
				.getFullList({ sort: "-created", fields: LIST_FIELDS });
			return {
				status: "ready",
				reports,
				email: String(pb.authStore.record?.email ?? ""),
			};
		} catch (err) {
			// A token that's expired or revoked server-side still looks valid
			// locally; drop it and ask again rather than showing an error page.
			if (
				(err as { status?: number }).status === 401 ||
				(err as { status?: number }).status === 403
			) {
				pb.authStore.clear();
				return { status: "signed-out", reports: [] };
			}
			throw err;
		}
	},
	head: () => ({
		meta: [
			{ title: "Reports · Standings" },
			// The page shows report links, so keep it private like the reports.
			{ name: "robots", content: "noindex, nofollow" },
		],
	}),
	component: ReportsRoute,
});

function ReportsRoute() {
	const data = Route.useLoaderData();
	return data.status === "ready" ? (
		<ReportList reports={data.reports} email={data.email} />
	) : (
		<SignIn />
	);
}

const dateFmt = new Intl.DateTimeFormat(undefined, {
	year: "numeric",
	month: "short",
	day: "numeric",
});
const timeFmt = new Intl.DateTimeFormat(undefined, {
	hour: "numeric",
	minute: "2-digit",
});
const numFmt = new Intl.NumberFormat();

/** PocketBase dates look like "2026-11-16 14:00:12.345Z". */
function parseDate(s: string | undefined) {
	if (!s) return null;
	const d = new Date(s.replace(" ", "T"));
	return Number.isNaN(d.getTime()) ? null : d;
}

function tokens(r: ReportsRecord) {
	const u = r.usage;
	if (!u) return null;
	return (u.input_tokens ?? 0) + (u.output_tokens ?? 0);
}

function ReportList({
	reports,
	email,
}: {
	reports: ReportsRecord[];
	email: string;
}) {
	const router = useRouter();
	const { pb } = Route.useRouteContext();
	const signOut = () => {
		pb.authStore.clear();
		router.invalidate();
	};

	return (
		<main className="page-wrap pb-16 pt-10">
			<div className="mb-8 flex flex-wrap items-end justify-between gap-4">
				<div>
					<h1 className="font-display text-4xl font-bold tracking-tight sm:text-5xl">
						Weekly reports
					</h1>
					<p className="mt-2 max-w-prose text-muted-foreground">
						{reports.length === 0
							? "Nothing generated yet."
							: `${reports.length} ${reports.length === 1 ? "report" : "reports"} generated, newest first.`}
					</p>
				</div>
				<div className="flex items-center gap-3 text-sm text-muted-foreground">
					<span className="truncate">{email}</span>
					<Button
						variant="outline"
						size="sm"
						className="bg-card"
						onClick={signOut}
					>
						<LogOut />
						Sign out
					</Button>
				</div>
			</div>

			<div className="card overflow-hidden">
				<Table>
					<TableHeader>
						<TableRow className="hover:bg-transparent">
							<TableHead>Generated</TableHead>
							<TableHead>Report</TableHead>
							<TableHead>Status</TableHead>
							<TableHead className="hidden md:table-cell">Recipients</TableHead>
							<TableHead className="hidden lg:table-cell text-right">
								Tokens
							</TableHead>
						</TableRow>
					</TableHeader>
					<TableBody>
						{reports.map((r) => {
							const created = parseDate(r.created);
							const sent = parseDate(r.sent_at);
							const total = tokens(r);
							return (
								<TableRow key={r.id}>
									<TableCell className="whitespace-nowrap align-top">
										{created ? (
											<>
												<div className="font-medium">
													{dateFmt.format(created)}
												</div>
												<div className="tnum text-xs text-muted-foreground">
													{timeFmt.format(created)}
												</div>
											</>
										) : (
											"–"
										)}
									</TableCell>
									<TableCell className="align-top whitespace-normal">
										<Link
											to="/reports/$slug"
											params={{ slug: r.slug }}
											className="row-link text-base font-semibold hover:text-primary"
										>
											{r.subject}
										</Link>
										{r.headline ? (
											<p className="mt-0.5 line-clamp-2 text-sm text-muted-foreground">
												{r.headline}
											</p>
										) : null}
									</TableCell>
									<TableCell className="align-top">
										<span
											className={cn(
												"inline-flex items-center rounded-full px-2 py-0.5 text-xs font-semibold",
												sent
													? "bg-gain/12 text-gain"
													: "bg-muted text-muted-foreground",
											)}
											title={
												sent
													? `Sent ${dateFmt.format(sent)} ${timeFmt.format(sent)}`
													: undefined
											}
										>
											{sent ? "Sent" : "Stored only"}
										</span>
									</TableCell>
									<TableCell className="hidden align-top text-sm text-muted-foreground md:table-cell whitespace-normal">
										{r.recipients || "–"}
									</TableCell>
									<TableCell
										className="tnum hidden align-top text-right text-muted-foreground lg:table-cell"
										title={r.model || undefined}
									>
										{total == null ? "–" : numFmt.format(total)}
									</TableCell>
								</TableRow>
							);
						})}
						{reports.length === 0 && (
							<TableRow className="hover:bg-transparent">
								<TableCell
									colSpan={5}
									className="py-12 text-center text-muted-foreground"
								>
									No reports yet. Generate one with{" "}
									<code>POST /api/aso/report/run</code>.
								</TableCell>
							</TableRow>
						)}
					</TableBody>
				</Table>
			</div>
		</main>
	);
}

function SignIn() {
	const router = useRouter();
	const { pb } = Route.useRouteContext();
	const [email, setEmail] = useState("");
	const [password, setPassword] = useState("");
	const [error, setError] = useState<string | null>(null);
	const [busy, setBusy] = useState(false);

	const submit = async (e: FormEvent) => {
		e.preventDefault();
		setBusy(true);
		setError(null);
		try {
			await pb.collection("_superusers").authWithPassword(email, password);
			await router.invalidate();
		} catch (err) {
			const status = (err as { status?: number }).status;
			setError(
				status === 400
					? "Those credentials didn't work."
					: "Couldn't reach the server. Try again.",
			);
		} finally {
			setBusy(false);
		}
	};

	return (
		<main className="page-wrap pb-16 pt-16">
			<form onSubmit={submit} className="card mx-auto max-w-sm space-y-4 p-8">
				<div>
					<h1 className="font-display text-3xl font-bold tracking-tight">
						Sign in to see reports
					</h1>
					<p className="mt-2 text-sm text-muted-foreground">
						Report links are private, so the list is only available to
						PocketBase superusers.
					</p>
				</div>
				<div className="space-y-1.5">
					<label htmlFor="report-email" className="block text-sm font-medium">
						Email
					</label>
					<Input
						id="report-email"
						type="email"
						autoComplete="username"
						required
						value={email}
						onChange={(e) => setEmail(e.target.value)}
					/>
				</div>
				<div className="space-y-1.5">
					<label
						htmlFor="report-password"
						className="block text-sm font-medium"
					>
						Password
					</label>
					<Input
						id="report-password"
						type="password"
						autoComplete="current-password"
						required
						value={password}
						onChange={(e) => setPassword(e.target.value)}
					/>
				</div>
				{error ? (
					<p role="alert" className="text-sm text-loss">
						{error}
					</p>
				) : null}
				<Button type="submit" className="w-full" disabled={busy}>
					{busy ? "Signing in…" : "Sign in"}
				</Button>
			</form>
		</main>
	);
}
