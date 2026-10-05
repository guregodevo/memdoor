// SHARE crypto, the page's half of cmd/cli/cmd/tui_share.go. A share link is
// /s/<id>#k=<key>: the key (32 bytes, base64url) never reaches a server; the
// sealed transcript at /api/share/<id> is nonce(12) ‖ AES-256-GCM ciphertext
// with "memdoor/share/v1" as additional data. Pure WebCrypto, no DOM:
// cmd/cli/cmd/tui_share_test.go runs this file in Node against the Go side.

import { b64u } from './remoteCrypto';

export const SHARE_AAD = 'memdoor/share/v1';

export type ShareLine = { role: 'user' | 'assistant'; text: string };
export type ShareDoc = { v: number; title: string; shared: string; messages: ShareLine[]; dropped?: number };

export async function openShare(key: string, sealed: Uint8Array): Promise<ShareDoc> {
  const raw = b64u.dec(key);
  if (raw.length !== 32) throw new Error('not a share key');
  const aes = await crypto.subtle.importKey('raw', raw as BufferSource, 'AES-GCM', false, ['decrypt']);
  const plain = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: sealed.slice(0, 12) as BufferSource, additionalData: new TextEncoder().encode(SHARE_AAD) },
    aes,
    sealed.slice(12) as BufferSource,
  );
  return JSON.parse(new TextDecoder().decode(plain)) as ShareDoc;
}
