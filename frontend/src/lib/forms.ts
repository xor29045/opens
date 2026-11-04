import type { FieldErrors, FieldValues, Resolver } from 'react-hook-form';
import { z } from 'zod';

/**
 * react-hook-form resolver backed directly by a Zod schema. Zod messages are
 * error codes (see messageFor in lib/api/problem.ts), so they can be shown
 * with the same French texts as the API.
 */
export function zodResolver<T extends FieldValues>(schema: z.ZodType<T>): Resolver<T> {
  return async (values) => {
    const result = schema.safeParse(values);
    if (result.success) return { values: result.data, errors: {} };
    const errors: Record<string, { type: string; message: string }> = {};
    for (const issue of result.error.issues) {
      const key = issue.path.join('.');
      if (!errors[key]) errors[key] = { type: issue.code, message: issue.message };
    }
    return { values: {} as T, errors: errors as FieldErrors<T> } as never;
  };
}
