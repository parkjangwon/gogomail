// WebAuthn browser integration helpers.
// Handles base64url ↔ ArrayBuffer conversion and navigator.credentials calls.

function base64URLToBuffer(str: string): ArrayBuffer {
  const padded = str.padEnd(str.length + ((4 - (str.length % 4)) % 4), '=');
  const binary = atob(padded.replace(/-/g, '+').replace(/_/g, '/'));
  const buf = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) buf[i] = binary.charCodeAt(i);
  return buf.buffer;
}

function bufferToBase64URL(buf: ArrayBuffer | Uint8Array): string {
  const bytes = buf instanceof Uint8Array ? buf : new Uint8Array(buf);
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=/g, '');
}

// Coerce JSON creation options from the backend into the typed form the browser API expects.
export function coerceCreationOptions(raw: Record<string, unknown>): PublicKeyCredentialCreationOptions {
  const pk = (raw.publicKey ?? raw) as Record<string, unknown>;
  const user = pk.user as Record<string, unknown>;
  const excludeCredentials = (pk.excludeCredentials as Array<Record<string, unknown>> | undefined) ?? [];

  return {
    ...(pk as object),
    challenge: base64URLToBuffer(pk.challenge as string),
    user: {
      ...(user as object),
      id: base64URLToBuffer(user.id as string),
    },
    excludeCredentials: excludeCredentials.map((c) => ({
      ...(c as object),
      id: base64URLToBuffer(c.id as string),
    })),
  } as PublicKeyCredentialCreationOptions;
}

// Coerce JSON request options from the backend into the typed form the browser API expects.
export function coerceRequestOptions(raw: Record<string, unknown>): PublicKeyCredentialRequestOptions {
  const pk = (raw.publicKey ?? raw) as Record<string, unknown>;
  const allowCredentials = (pk.allowCredentials as Array<Record<string, unknown>> | undefined) ?? [];

  return {
    ...(pk as object),
    challenge: base64URLToBuffer(pk.challenge as string),
    allowCredentials: allowCredentials.map((c) => ({
      ...(c as object),
      id: base64URLToBuffer(c.id as string),
    })),
  } as PublicKeyCredentialRequestOptions;
}

// Serialize a PublicKeyCredential to plain JSON (base64url-encode all ArrayBuffers).
export function credentialToJSON(cred: PublicKeyCredential): Record<string, unknown> {
  const response = cred.response;

  if (response instanceof AuthenticatorAttestationResponse) {
    return {
      id: cred.id,
      rawId: bufferToBase64URL(cred.rawId),
      type: cred.type,
      response: {
        attestationObject: bufferToBase64URL(response.attestationObject),
        clientDataJSON: bufferToBase64URL(response.clientDataJSON),
      },
    };
  }

  if (response instanceof AuthenticatorAssertionResponse) {
    return {
      id: cred.id,
      rawId: bufferToBase64URL(cred.rawId),
      type: cred.type,
      response: {
        authenticatorData: bufferToBase64URL(response.authenticatorData),
        clientDataJSON: bufferToBase64URL(response.clientDataJSON),
        signature: bufferToBase64URL(response.signature),
        userHandle: response.userHandle ? bufferToBase64URL(response.userHandle) : null,
      },
    };
  }

  throw new Error('Unknown credential response type');
}

export function isWebAuthnSupported(): boolean {
  return typeof window !== 'undefined' && !!window.PublicKeyCredential && !!navigator.credentials;
}
