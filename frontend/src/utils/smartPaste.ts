import { ParsedTarget } from '../types/ssh';

const HOST_REGEX = /^(?:[a-zA-Z0-9-]+(?:\.[a-zA-Z0-9-]+)+|\d{1,3}(?:\.\d{1,3}){3}|localhost)$/;

export function parseSmartPaste(raw: string): ParsedTarget | null {
  let input = raw.trim();
  if (!input) return null;

  let port = 22;
  let username = 'root';

  if (/^ssh\s+/i.test(input)) {
    input = input.replace(/^ssh\s+/i, '').trim();
    const portMatch = input.match(/(?:^|\s)-p\s*(\d{1,5})(?:\s|$)/);
    if (portMatch) {
      port = Number(portMatch[1]);
      input = input.replace(portMatch[0], ' ').trim();
    }
    const tokens = input.split(/\s+/);
    const targetToken = tokens.find((t) => !t.startsWith('-') && (t.includes('@') || t.includes('.')));
    if (!targetToken) return null;
    input = targetToken;
  }

  if (input.includes('@')) {
    const [u, rest] = input.split('@');
    if (u) username = u;
    input = rest || '';
  }

  if (input.includes(':')) {
    const [h, p] = input.split(':');
    const parsedPort = Number(p);
    if (!Number.isInteger(parsedPort) || parsedPort < 1 || parsedPort > 65535) {
      return null;
    }
    port = parsedPort;
    input = h;
  }

  if (!HOST_REGEX.test(input)) return null;
  return { username, hostname: input, port };
}
