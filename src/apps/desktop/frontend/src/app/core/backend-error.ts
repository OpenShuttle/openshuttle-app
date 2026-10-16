/** Error sent by Go as the "cause" of a rejected call: a stable code. */
export interface BackendError {
  code: string;
  /** 1-based set number, for score errors about one set. */
  set?: number;
}

export function backendError(err: unknown): BackendError {
  const cause = (err as { cause?: { code?: unknown; set?: unknown } } | null)?.cause;
  if (typeof cause?.code !== 'string') {
    return { code: 'INTERNAL' };
  }
  return typeof cause.set === 'number' ? { code: cause.code, set: cause.set } : { code: cause.code };
}
