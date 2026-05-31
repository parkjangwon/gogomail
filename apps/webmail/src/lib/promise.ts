export function ignoreNonCritical(promise: Promise<unknown>, context: string): void {
  void promise.catch((error: unknown) => {
    const message = error instanceof Error ? error.message : String(error);
    console.warn('non-critical async task failed', { context, error: message });
  });
}
