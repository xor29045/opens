import { useState, type ReactNode } from 'react';
import { Button, Dialog, Field, Stack, Table, Text, Textarea, Portal, Box } from '@chakra-ui/react';
import { EmptyState, PageLoader, ErrorState } from '../../components/ui';
import { describeError } from '../../lib/api/problem';

export interface Column<T> {
  key: string;
  header: string;
  cell: (row: T) => ReactNode;
}

/** Simple accessible data table. Rows are rendered as given (already paginated). */
export function DataTable<T extends { id?: string | number }>({
  columns, rows, empty, caption,
}: { columns: Column<T>[]; rows: T[]; empty: string; caption: string }) {
  if (rows.length === 0) return <EmptyState title={empty} />;
  return (
    <Box overflowX="auto">
      <Table.Root size="sm" variant="outline" interactive>
        <Table.Caption captionSide="top" textAlign="left" srOnly>{caption}</Table.Caption>
        <Table.Header>
          <Table.Row>
            {columns.map((c) => (
              <Table.ColumnHeader key={c.key} scope="col">{c.header}</Table.ColumnHeader>
            ))}
          </Table.Row>
        </Table.Header>
        <Table.Body>
          {rows.map((r, i) => (
            <Table.Row key={String(r.id ?? i)}>
              {columns.map((c) => (
                <Table.Cell key={c.key}>{c.cell(r)}</Table.Cell>
              ))}
            </Table.Row>
          ))}
        </Table.Body>
      </Table.Root>
    </Box>
  );
}

/** Wraps a query result: loading, error (with retry) or the content. */
export function QueryBlock({ isPending, isError, error, onRetry, children }: {
  isPending: boolean; isError: boolean; error: unknown; onRetry: () => void; children: ReactNode;
}) {
  if (isPending) return <PageLoader />;
  if (isError) return <ErrorState message={describeError(error)} onRetry={onRetry} />;
  return <>{children}</>;
}

/**
 * Button that opens a confirmation dialog. When `withReason` is set, a
 * non-empty reason is required (RM-60): it is stored in the audit trail.
 */
export function ActionDialog({ trigger, title, description, confirmLabel, withReason = false, danger = false, onConfirm }: {
  trigger: string;
  title: string;
  description: string;
  confirmLabel: string;
  withReason?: boolean;
  danger?: boolean;
  onConfirm: (reason: string) => Promise<unknown>;
}) {
  const [open, setOpen] = useState(false);
  const [reason, setReason] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const submit = async () => {
    if (withReason && reason.trim() === '') {
      setError('Le motif est obligatoire.');
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await onConfirm(reason.trim());
      setOpen(false);
      setReason('');
    } catch (e) {
      setError(describeError(e));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Dialog.Root open={open} onOpenChange={(d) => setOpen(d.open)} placement="center" role="alertdialog">
      <Dialog.Trigger asChild>
        <Button size="xs" variant={danger ? 'outline' : 'subtle'} colorPalette={danger ? 'red' : 'gray'}>{trigger}</Button>
      </Dialog.Trigger>
      <Portal>
        <Dialog.Backdrop />
        <Dialog.Positioner>
          <Dialog.Content>
            <Dialog.Header>
              <Dialog.Title>{title}</Dialog.Title>
            </Dialog.Header>
            <Dialog.Body>
              <Stack gap="3">
                <Text>{description}</Text>
                {withReason && (
                  <Field.Root required>
                    <Field.Label>Motif</Field.Label>
                    <Textarea value={reason} onChange={(e) => setReason(e.target.value)} rows={3} maxLength={500} />
                    <Field.HelperText>Conservé dans le journal d’audit.</Field.HelperText>
                  </Field.Root>
                )}
                {error && <Text color="danger" role="alert">{error}</Text>}
              </Stack>
            </Dialog.Body>
            <Dialog.Footer>
              <Dialog.ActionTrigger asChild>
                <Button variant="outline" disabled={busy}>Annuler</Button>
              </Dialog.ActionTrigger>
              <Button colorPalette={danger ? 'red' : 'blue'} onClick={submit} loading={busy}>{confirmLabel}</Button>
            </Dialog.Footer>
          </Dialog.Content>
        </Dialog.Positioner>
      </Portal>
    </Dialog.Root>
  );
}
