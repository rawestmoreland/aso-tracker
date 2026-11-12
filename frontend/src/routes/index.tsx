import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '#/components/ui/table';
import { Button } from '#/components/ui/button';
import { Input } from '#/components/ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '#/components/ui/select';
import type { LatestRanksRecord } from '#/lib/pocketbase-types';
import { Link, createFileRoute } from '@tanstack/react-router';
import {
  columnFilteringFeature,
  createFilteredRowModel,
  createPaginatedRowModel,
  createSortedRowModel,
  filterFn_includesString,
  rowPaginationFeature,
  rowSortingFeature,
  tableFeatures,
  useTable,
  type ColumnDef,
} from '@tanstack/react-table';

const RANK_FILTERS = {
  all: { label: 'All ranks', test: () => true },
  top10: { label: 'Top 10', test: (r: number) => r >= 1 && r <= 10 },
  top50: { label: 'Top 50', test: (r: number) => r >= 1 && r <= 50 },
  ranked: { label: 'Ranked (≤200)', test: (r: number) => r >= 1 },
  unranked: { label: 'Unranked (>200)', test: (r: number) => r === 0 },
} as const;
type RankFilter = keyof typeof RANK_FILTERS;

const PAGE_SIZES = [10, 25, 50, 100];

export const Route = createFileRoute('/')({
  ssr: false,
  loader: async ({ context }) => {
    const data = await context.pb
      .collection<LatestRanksRecord>('latest_ranks')
      .getFullList();

    return data ?? [];
  },
  component: App,
});

function App() {
  const data = Route.useLoaderData();

  const change = (row: LatestRanksRecord) => {
    const prev = row.rank_7d ?? row.rank_first;

    if (prev == null || typeof row.rank !== 'number') return null;
    if (prev === 0 && row.rank === 0) return null;
    if (prev === 0) return { label: 'new', delta: null };
    if (row.rank === 0) return { label: 'dropped', delta: null };
    return { label: null, delta: (prev as number) - row.rank };
  };

  const features = tableFeatures({
    rowSortingFeature,
    columnFilteringFeature,
    rowPaginationFeature,
    sortedRowModel: createSortedRowModel(),
    filteredRowModel: createFilteredRowModel(),
    paginatedRowModel: createPaginatedRowModel(),
    filterFns: { includesString: filterFn_includesString },
  });

  const columns: Array<ColumnDef<typeof features, LatestRanksRecord>> = [
    {
      accessorKey: 'term',
      header: 'Keyword',
      filterFn: 'includesString',
      cell: ({ row }) => (
        <Link
          to='/keywords/$keywordId'
          params={{ keywordId: String(row.original.keyword) }}
          className='underline'
        >
          {row.original.term}
        </Link>
      ),
    },
    {
      id: 'rank',
      accessorFn: (row) => row.rank || undefined,
      sortUndefined: 'last',
      sortDescFirst: false,
      header: 'Rank',
      filterFn: (row, _columnId, value: RankFilter) =>
        RANK_FILTERS[value].test(Number(row.original.rank) || 0),
      sortFn: (a, b, columnId) =>
        a.getValue<number>(columnId) - b.getValue<number>(columnId),
      cell: ({ row }) => (row.original.rank ? `#${row.original.rank}` : '>200'),
    },
    {
      accessorKey: 'rank_7d',
      header: '7d',
      cell: (info) => {
        const v = info.getValue();
        return v == null ? '–' : v === 0 ? '>200' : `#${v}`;
      },
    },
    {
      id: 'change',
      accessorFn: (row) => change(row),
      header: 'Change',
      cell: ({ getValue }) => {
        const c = getValue<ReturnType<typeof change>>();
        if (!c) return '–';
        if (c.label === 'new') return 'new';
        if (c.label === 'dropped') return 'dropped';
        return c.delta! > 0
          ? `▲ ${c.delta}`
          : c.delta! < 0
            ? `▼ ${-c.delta!}`
            : '–';
      },
      sortFn: (a, b) =>
        (a.getValue<any>('change')?.delta ?? 0) -
        (b.getValue<any>('change')?.delta ?? 0),
    },
  ];

  const table = useTable({
    key: 'table',
    features,
    columns,
    data: data ?? [],
    initialState: {
      sorting: [{ id: 'rank', desc: false }],
      pagination: { pageIndex: 0, pageSize: 25 },
    },
    enableSortingRemoval: false,
  });

  const termColumn = table.getColumn('term')!;
  const rankColumn = table.getColumn('rank')!;
  const { pageIndex, pageSize } = table.state.pagination;
  const filteredCount = table.getFilteredRowModel().rows.length;
  const from = filteredCount === 0 ? 0 : pageIndex * pageSize + 1;
  const to = Math.min((pageIndex + 1) * pageSize, filteredCount);

  if (!data) return null;

  return (
    <main className='page-wrap px-4 pb-8 pt-14'>
      <div className='mb-4 flex flex-wrap items-center gap-3'>
        <Input
          type='search'
          placeholder='Search keywords…'
          value={(termColumn.getFilterValue() as string) ?? ''}
          onChange={(e) => termColumn.setFilterValue(e.target.value)}
          className='w-56'
        />
        <Select
          value={(rankColumn.getFilterValue() as RankFilter) ?? 'all'}
          onValueChange={(v) =>
            rankColumn.setFilterValue(v === 'all' ? undefined : v)
          }
        >
          <SelectTrigger className='w-44'>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {Object.entries(RANK_FILTERS).map(([key, { label }]) => (
              <SelectItem key={key} value={key}>
                {label}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              {headerGroup.headers.map((header) => (
                <TableHead
                  key={header.id}
                  onClick={header.column.getToggleSortingHandler()}
                  className={
                    header.column.getCanSort()
                      ? 'cursor-pointer select-none'
                      : ''
                  }
                >
                  {header.isPlaceholder ? null : (
                    <table.FlexRender header={header} />
                  )}
                  {{ asc: ' ↑', desc: ' ↓' }[
                    header.column.getIsSorted() as string
                  ] ?? null}
                </TableHead>
              ))}
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.map((row) => (
            <TableRow key={row.id}>
              {row.getAllCells().map((cell) => (
                <TableCell key={cell.id}>
                  <table.FlexRender cell={cell} />
                </TableCell>
              ))}
            </TableRow>
          ))}
          {table.getRowModel().rows.length === 0 && (
            <TableRow>
              <TableCell colSpan={columns.length} className='text-center'>
                No keywords match your filters.
              </TableCell>
            </TableRow>
          )}
        </TableBody>
      </Table>
      <div className='mt-4 flex flex-wrap items-center justify-between gap-3 text-sm'>
        <div>
          {from}–{to} of {filteredCount}
        </div>
        <div className='flex items-center gap-2'>
          <Select
            value={String(pageSize)}
            onValueChange={(v) => table.setPageSize(Number(v))}
          >
            <SelectTrigger size='sm' className='w-32'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {PAGE_SIZES.map((size) => (
                <SelectItem key={size} value={String(size)}>
                  {size} / page
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button
            variant='outline'
            size='sm'
            onClick={() => table.previousPage()}
            disabled={!table.getCanPreviousPage()}
          >
            Previous
          </Button>
          <span>
            Page {pageIndex + 1} of {Math.max(table.getPageCount(), 1)}
          </span>
          <Button
            variant='outline'
            size='sm'
            onClick={() => table.nextPage()}
            disabled={!table.getCanNextPage()}
          >
            Next
          </Button>
        </div>
      </div>
    </main>
  );
}
