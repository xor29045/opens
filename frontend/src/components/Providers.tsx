import { useState, type ReactNode } from 'react';
import { ChakraProvider } from '@chakra-ui/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { system } from '../theme/system';

function makeQueryClient(): QueryClient {
  return new QueryClient({
    defaultOptions: {
      queries: {
        staleTime: 30_000,
        // Client errors (4xx) are not retried; network and 5xx errors twice.
        retry: (count, err) => {
          const status = (err as { status?: number }).status ?? 0;
          return count < 2 && (status === 0 || status >= 500);
        },
      },
      mutations: { retry: false },
    },
  });
}

// One provider tree per island (each Astro island is an independent React
// root). The query cache is created per root so that it is never shared
// between SSR requests of different visitors.
export default function Providers({ children }: { children: ReactNode }) {
  const [queryClient] = useState(makeQueryClient);
  return (
    <ChakraProvider value={system}>
      <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>
    </ChakraProvider>
  );
}
