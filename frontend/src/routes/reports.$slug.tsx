import { createFileRoute, Link, notFound } from "@tanstack/react-router";
import { ReportPage } from "#/components/report/ReportPage";
import { Button } from "#/components/ui/button";
import type { PublicReportsRecord } from "#/lib/pocketbase-types";
import { isReportView, type ReportView } from "#/lib/report";

export const Route = createFileRoute("/reports/$slug")({
	ssr: false,
	loader: async ({ context, params }) => {
		const { pb } = context;
		// `public_reports` only matches a record when the request carries its slug
		// (that is its list rule), so a report can be opened by whoever has the
		// link but can't be listed or enumerated.
		const page = await pb
			.collection<PublicReportsRecord<unknown>>("public_reports")
			.getList(1, 1, { query: { slug: params.slug }, requestKey: null });
		const report = page.items[0];
		if (!report) throw notFound();
		// Where the backend serves its own copy of this report (the only copy
		// that exists for reports made before the web view was added).
		const serverCopy = `${pb.baseURL}/api/aso/report/view/${encodeURIComponent(params.slug)}`;
		return { report, serverCopy };
	},
	head: ({ loaderData }) => ({
		meta: [
			{ title: loaderData?.report.subject ?? "Weekly report" },
			// A report link is a private link: keep it out of search and don't let the
			// slug travel in Referer headers.
			{ name: "robots", content: "noindex, nofollow, noarchive" },
			{ name: "referrer", content: "no-referrer" },
		],
	}),
	notFoundComponent: ReportNotFound,
	component: ReportRoute,
});

function ReportRoute() {
	const { report, serverCopy } = Route.useLoaderData();
	if (!isReportView(report.view)) {
		return <LegacyReport serverCopy={serverCopy} />;
	}
	return <ReportPage view={report.view as ReportView} />;
}

function Notice({
	title,
	children,
}: {
	title: string;
	children: React.ReactNode;
}) {
	return (
		<main className="page-wrap pb-16 pt-16">
			<div className="card mx-auto max-w-lg p-8 text-center">
				<h1 className="font-display text-3xl font-bold tracking-tight">
					{title}
				</h1>
				<div className="mt-3 text-muted-foreground">{children}</div>
			</div>
		</main>
	);
}

function ReportNotFound() {
	return (
		<Notice title="Report not found">
			<p>This link doesn't match a report. Check that you copied all of it.</p>
			<Button asChild className="mt-6">
				<Link to="/">Back to keywords</Link>
			</Button>
		</Notice>
	);
}

/** Reports created before the web view existed only have the server copy. */
function LegacyReport({ serverCopy }: { serverCopy: string }) {
	return (
		<Notice title="Open the original copy">
			<p>
				This report was generated before reports could be shown on this page, so
				it's only available as the original email layout.
			</p>
			<Button asChild className="mt-6">
				<a href={serverCopy}>Open report</a>
			</Button>
		</Notice>
	);
}
