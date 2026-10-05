// REMOTE CONTROL crypto, the browser's half of cmd/cli/cmd/tui_remote.go.
// The link key (after # in /r/<id>#k=<key>) derives, with HKDF-SHA256 under
// the same salt and labels as the terminal:
//   info "session"      → the AES-256-GCM key every frame is sealed with
//   info "browser-auth" → the proof the relay checks (?auth=)
//   info "write"        → the key a page seals a DRIVING frame with
// A watch-only link (/r/<id>#v=<token>) carries base64url(session key ‖
// proof) instead of the link key: it reads and joins, and cannot derive the
// write key, so the terminal drops anything it sends but a request to see.
// A frame's payload is base64url(nonce ‖ ciphertext) with a random 12-byte
// nonce, and the direction ("t2b" terminal→browser, "b2t" browser→terminal)
// is bound as additional data, so a reflected frame never opens.
//
// Pure WebCrypto, no DOM: cmd/cli/cmd/tui_remote_test.go runs this file in
// Node against the Go side's vectors.

export const REMOTE_SALT = 'memdoor/remote-control/v1';
export const TO_BROWSER = 't2b';
export const TO_TERMINAL = 'b2t';

const te = new TextEncoder();
const td = new TextDecoder();

export const b64u = {
  enc(bytes: Uint8Array): string {
    let bin = '';
    for (const b of bytes) bin += String.fromCharCode(b);
    return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  },
  dec(s: string): Uint8Array {
    const std = s.replace(/-/g, '+').replace(/_/g, '/');
    const bin = atob(std + '='.repeat((4 - (std.length % 4)) % 4));
    return Uint8Array.from(bin, (c) => c.charCodeAt(0));
  },
};

async function hkdf(key: string, info: string, len: number): Promise<Uint8Array> {
  const ikm = await crypto.subtle.importKey('raw', te.encode(key), 'HKDF', false, ['deriveBits']);
  const bits = await crypto.subtle.deriveBits(
    { name: 'HKDF', hash: 'SHA-256', salt: te.encode(REMOTE_SALT), info: te.encode(info) },
    ikm,
    len * 8,
  );
  return new Uint8Array(bits);
}

// write is absent on a watch-only link: that page cannot drive.
export type RemoteKeys = { aes: CryptoKey; proof: string; write?: CryptoKey };

function aesKey(raw: Uint8Array): Promise<CryptoKey> {
  return crypto.subtle.importKey('raw', raw as BufferSource, 'AES-GCM', false, ['encrypt', 'decrypt']);
}

export async function deriveRemote(key: string): Promise<RemoteKeys> {
  const [aeadKey, proof, writeKey] = await Promise.all([
    hkdf(key, 'session', 32),
    hkdf(key, 'browser-auth', 32),
    hkdf(key, 'write', 32),
  ]);
  return { aes: await aesKey(aeadKey), proof: b64u.enc(proof), write: await aesKey(writeKey) };
}

// deriveView reads a watch-only token: the session key and the proof, no more.
export async function deriveView(token: string): Promise<RemoteKeys> {
  const raw = b64u.dec(token);
  if (raw.length !== 64) throw new Error('not a watch-only token');
  return { aes: await aesKey(raw.slice(0, 32)), proof: b64u.enc(raw.slice(32)) };
}

// seal: a frame to the terminal goes under the write key when the page has
// one (it may drive), under the session key otherwise (it may only watch).
export async function seal(k: RemoteKeys, dir: string, plain: string, nonce?: Uint8Array): Promise<string> {
  const iv = nonce ?? crypto.getRandomValues(new Uint8Array(12));
  const key = dir === TO_TERMINAL && k.write ? k.write : k.aes;
  const ct = new Uint8Array(
    await crypto.subtle.encrypt({ name: 'AES-GCM', iv: iv as BufferSource, additionalData: te.encode(dir) }, key, te.encode(plain)),
  );
  const out = new Uint8Array(iv.length + ct.length);
  out.set(iv);
  out.set(ct, iv.length);
  return b64u.enc(out);
}

// open throws when the frame was not sealed with this key for this direction.
export async function open(k: RemoteKeys, dir: string, payload: string): Promise<string> {
  const raw = b64u.dec(payload);
  const plain = await crypto.subtle.decrypt(
    { name: 'AES-GCM', iv: raw.slice(0, 12) as BufferSource, additionalData: te.encode(dir) },
    k.aes,
    raw.slice(12) as BufferSource,
  );
  return td.decode(plain);
}
