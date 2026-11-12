import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '#/components/ui/table';
import type {
  LatestRanksRecord,
  RankHistoryRecord,
} from '#/lib/pocketbase-types';
import { Link, createFileRoute } from '@tanstack/react-router';
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';

export const Route = createFileRoute('/keywords/$keywordId')({
  ssr: false,
  loader: async ({ context, params }) => {
    const { pb } = context;
    const [history, apps] = await Promise.all([
      pb.collection<RankHistoryRecord>('rank_history').getFullList({
        filter: pb.filter('keyword = {:id}', { id: params.keywordId }),
        sort: 'checked_at',
      }),
      // latest_ranks has one row per tracked app for this keyword, which gives
      // us the app names and is_mine without needing access to tracked_apps.
      pb.collection<LatestRanksRecord>('latest_ranks').getFullList({
        filter: pb.filter('keyword = {:id}', { id: params.keywordId }),
      }),
    ]);
    return { history, apps };
  },
  component: KeywordDetails,
});

const COLORS = [
  '#2563eb',
  '#dc2626',
  '#16a34a',
  '#d97706',
  '#9333ea',
  '#0891b2',
];
const MAX_RANK = 200; // iTunes search returns at most 200 results

const day = (iso: string) => iso.slice(0, 10);

function KeywordDetails() {
  const { history, apps } = Route.useLoaderData();

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

  return (
    <main className='page-wrap px-4 pb-8 pt-14'>
      <Link to='/' className='text-sm'>
        ← All keywords
      </Link>
      <h1 className='mb-6 mt-2 text-2xl font-semibold'>
        {term ?? 'Keyword'}
        {country ? (
          <span className='ml-2 text-sm uppercase opacity-60'>{country}</span>
        ) : null}
      </h1>

      <div className='h-96 w-full'>
        <ResponsiveContainer width='100%' height='100%'>
          <LineChart data={chartData}>
            <CartesianGrid strokeDasharray='3 3' opacity={0.3} />
            <XAxis dataKey='date' />
            <YAxis
              reversed
              allowDecimals={false}
              domain={[1, worst]}
              label={{ value: 'Rank', angle: -90, position: 'insideLeft' }}
            />
            <Tooltip
              formatter={(v, name) => [
                v == null ? '>200' : `#${v}`,
                appName.get(String(name)) ?? name,
              ]}
            />
            <Legend formatter={(v) => appName.get(String(v)) ?? v} />
            {appList.map((a, i) => (
              <Line
                key={a.id}
                dataKey={a.id}
                name={a.id}
                stroke={COLORS[i % COLORS.length]}
                strokeWidth={a.isMine ? 3 : 1.5}
                connectNulls={false}
                dot={{ r: 3 }}
                isAnimationActive={false}
              />
            ))}
            {/* TODO: <ReferenceLine x='YYYY-MM-DD' /> for title/subtitle changes */}
          </LineChart>
        </ResponsiveContainer>
      </div>

      <h2 className='mb-2 mt-8 text-lg font-semibold'>Recent checks</h2>
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Checked</TableHead>
            <TableHead>App</TableHead>
            <TableHead>Rank</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {checks.map((c) => (
            <TableRow key={c.id}>
              <TableCell>
                {new Date(c.checked_at.replace(' ', 'T')).toLocaleString()}
              </TableCell>
              <TableCell>{appName.get(c.app) ?? c.app}</TableCell>
              <TableCell>{c.rank ? `#${c.rank}` : '>200'}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </main>
  );
}
