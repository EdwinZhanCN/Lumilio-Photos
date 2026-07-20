export const baseURL = process.env.LUMILIO_E2E_BASE_URL ?? "http://127.0.0.1:16657";

type RequestInitLike = {
  method?: string;
  body?: string;
  token?: string;
};

/** Thin JSON client for fixture setup against the real API. */
export async function api<T = unknown>(pathname: string, init: RequestInitLike = {}): Promise<T> {
  const response = await fetch(`${baseURL}${pathname}`, {
    method: init.method,
    body: init.body,
    headers: {
      "content-type": "application/json",
      ...(init.token ? { authorization: `Bearer ${init.token}` } : {}),
    },
  });
  const body: unknown = await response.json().catch(() => ({}));
  if (!response.ok) {
    throw new Error(
      `${init.method ?? "GET"} ${pathname}: ${response.status} ${JSON.stringify(body)}`,
    );
  }
  return body as T;
}
