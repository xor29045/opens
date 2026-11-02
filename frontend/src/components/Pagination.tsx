import { HStack, Button, Text } from '@chakra-ui/react';

export default function Pagination({ page, totalPages, onPage }: { page: number; totalPages: number; onPage: (p: number) => void }) {
  if (totalPages <= 1) return null;
  return (
    <HStack justify="center" gap="3" mt="6" aria-label="Pagination">
      <Button size="sm" variant="outline" disabled={page <= 1} onClick={() => onPage(page - 1)}>
        Précédent
      </Button>
      <Text fontSize="sm" color="fg.muted" aria-live="polite">
        Page {page} sur {totalPages}
      </Text>
      <Button size="sm" variant="outline" disabled={page >= totalPages} onClick={() => onPage(page + 1)}>
        Suivant
      </Button>
    </HStack>
  );
}
