export interface ErrorEnvelope {
  error: { code: string; message: string; fields?: Record<string, string> };
}

export function errorBody(code: string, message: string): ErrorEnvelope {
  return { error: { code, message } };
}
