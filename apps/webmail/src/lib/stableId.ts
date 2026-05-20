let fallbackCounter = 0;

function fallbackEntropy(): string {
  fallbackCounter = (fallbackCounter + 1) % Number.MAX_SAFE_INTEGER;

  const cryptoObj = globalThis.crypto;
  if (cryptoObj?.getRandomValues) {
    const bytes = new Uint32Array(2);
    cryptoObj.getRandomValues(bytes);
    return `${bytes[0].toString(36)}${bytes[1].toString(36)}${fallbackCounter.toString(36)}`;
  }

  return `${Date.now().toString(36)}${fallbackCounter.toString(36)}`;
}

export function stableId(prefix?: string): string {
  const id = globalThis.crypto?.randomUUID?.() ?? fallbackEntropy();
  return prefix ? `${prefix}-${id}` : id;
}

export function calendarUID(): string {
  return `${stableId()}@gogomail`;
}
