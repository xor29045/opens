import type { ReactNode } from 'react';
import { Badge, Box, Button, Center, Heading, HStack, Spinner, Stack, Text } from '@chakra-ui/react';

export function PageLoader({ label = 'Chargement…' }: { label?: string }) {
  return (
    <Center minH="40vh" role="status" aria-live="polite">
      <HStack>
        <Spinner color="accent.solid" />
        <Text color="fg.muted">{label}</Text>
      </HStack>
    </Center>
  );
}

export function EmptyState({ title, children }: { title: string; children?: ReactNode }) {
  return (
    <Stack align="center" gap="3" py="12" textAlign="center">
      <Heading size="md">{title}</Heading>
      {children && <Text color="fg.muted">{children}</Text>}
    </Stack>
  );
}

export function ErrorState({ message, onRetry }: { message: string; onRetry?: () => void }) {
  return (
    <Stack role="alert" gap="3" py="8" align="flex-start" borderWidth="1px" borderColor="danger" borderRadius="md" p="4">
      <Text color="danger">{message}</Text>
      {onRetry && (
        <Button size="sm" variant="outline" onClick={onRetry}>
          Réessayer
        </Button>
      )}
    </Stack>
  );
}

const statusColors: Record<string, string> = {
  PENDING: 'orange', DRAFT: 'gray',
  ACTIVE: 'green', ARCHIVED: 'gray',
  SUSPENDED: 'red', DELETED: 'red', PAST_DUE: 'orange', CANCELED: 'gray', INCOMPLETE: 'orange', TRIALING: 'blue',
  PAID: 'green', PARTIAL: 'orange', FAILED: 'red', EXPIRED: 'gray', REFUNDED: 'purple', PENDING_PAYMENT: 'orange',
};

const statusLabels: Record<string, string> = {
  PENDING: 'En attente', DRAFT: 'Brouillon',
  ACTIVE: 'Actif', ARCHIVED: 'Archivé',
  SUSPENDED: 'Suspendu', DELETED: 'Supprimé', PAST_DUE: 'Impayé (grâce)', CANCELED: 'Résilié',
  INCOMPLETE: 'Paiement en attente', TRIALING: 'Essai', PAID: 'Payé', PARTIAL: 'Partiel',
  FAILED: 'Échoué', EXPIRED: 'Expiré', REFUNDED: 'Remboursé',
};

export function StatusBadge({ value }: { value: string }) {
  return (
    <Badge colorPalette={statusColors[value] ?? 'gray'} variant="subtle" borderRadius="md">
      {statusLabels[value] ?? value}
    </Badge>
  );
}

export function Section({ title, children, action }: { title: string; children: ReactNode; action?: ReactNode }) {
  return (
    <Box as="section" borderWidth="1px" borderColor="border" borderRadius="lg" p="5" bg="bg">
      <HStack justify="space-between" mb="4">
        <Heading size="md">{title}</Heading>
        {action}
      </HStack>
      {children}
    </Box>
  );
}
