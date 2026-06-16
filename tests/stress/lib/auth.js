import http from 'k6/http';
import { check, fail } from 'k6';
import { BASE_URL, EMAIL, PASSWORD } from '../config.js';

/**
 * Registers the stress user. Safe to call if the user already exists (409 is ignored).
 * Call once in setup() before any login attempts.
 */
export function ensureStressUser() {
  const res = http.post(
    `${BASE_URL}/auth/register`,
    JSON.stringify({ username: 'stress', email: EMAIL, password: PASSWORD }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  if (res.status !== 200 && res.status !== 201 && res.status !== 409) {
    fail(`ensureStressUser failed (${res.status}): ${res.body}`);
  }
}

/**
 * Logs in and returns { accessToken, refreshToken }.
 */
export function login() {
  const res = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: EMAIL, password: PASSWORD }),
    { headers: { 'Content-Type': 'application/json' } },
  );

  const ok = check(res, { 'setup login 200': (r) => r.status === 200 });
  if (!ok) {
    fail(`Login failed (${res.status}): ${res.body}`);
  }

  const body = res.json();
  return { accessToken: body.access_token, refreshToken: body.refresh_token };
}

// Per-VU token state (module-level = VU-scoped in k6).
// Each VU gets its own JS context so there are no shared-state races.
let _token = null;
let _tokenExpiry = 0;

const TOKEN_TTL_MS = 270_000; // 4.5 min — slightly under the 300s JWT TTL
const REFRESH_SKEW = 30_000;  // re-login 30s before expiry

/**
 * Returns a valid Bearer token for the current VU, logging in (or
 * re-logging in) transparently when the token is absent or near expiry.
 */
export function getToken() {
  if (!_token || Date.now() >= _tokenExpiry - REFRESH_SKEW) {
    const { accessToken } = login();
    _token       = accessToken;
    _tokenExpiry = Date.now() + TOKEN_TTL_MS;
  }
  return _token;
}

/** Returns headers with Bearer token and JSON content type. */
export function authHeaders(token) {
  return {
    headers: {
      'Content-Type': 'application/json',
      Authorization: `Bearer ${token}`,
    },
  };
}
