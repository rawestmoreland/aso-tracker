import pocketbase from '#/lib/pocketbase';
import { QueryClient } from '@tanstack/react-query';

export function getContext() {
  const queryClient = new QueryClient();
  const pb = pocketbase;

  return {
    queryClient,
    pb,
  };
}
export default function TanstackQueryProvider() {}
