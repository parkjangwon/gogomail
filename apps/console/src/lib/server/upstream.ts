export async function fetchUpstreamOrNull(
  input: Parameters<typeof fetch>[0],
  init?: Parameters<typeof fetch>[1],
): Promise<Response | null> {
  try {
    return await fetch(input, init);
  } catch {
    return null;
  }
}

export async function readJSONOrDefault<T>(response: Response, fallback: T): Promise<T> {
  try {
    return await response.json() as T;
  } catch {
    return fallback;
  }
}
