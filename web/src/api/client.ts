export async function getJSON<T>(path: string): Promise<T> {
  const response = await fetch(path);
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
  return response.json() as Promise<T>;
}
