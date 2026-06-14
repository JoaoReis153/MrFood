/**
 * Smoke test — 1 VU, hits every endpoint once to verify the stack is up.
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api smoke.js
 */
import http from 'k6/http';
import { check } from 'k6';
import { BASE_URL } from './config.js';
import { login, authHeaders, ensureStressUser } from './lib/auth.js';
import { futureTimestamp } from './lib/utils.js';

export const options = {
  vus: 1,
  iterations: 1,
  thresholds: {
    checks: ['rate==1.0'],
  },
};

export function setup() {
  ensureStressUser();
}

export default function () {
  // Auth
  const { accessToken, refreshToken } = login();
  const h = authHeaders(accessToken);

  // Refresh
  const refreshRes = http.post(
    `${BASE_URL}/auth/refresh`,
    JSON.stringify({ refresh_token: refreshToken }),
    { headers: { 'Content-Type': 'application/json' } },
  );
  check(refreshRes, { 'refresh 200': (r) => r.status === 200 });

  // Restaurants
  const getRes = http.get(`${BASE_URL}/restaurants/1`, h);
  check(getRes, { 'get restaurant': (r) => r.status === 200 || r.status === 404 });

  const compareRes = http.get(`${BASE_URL}/restaurants/compare?restaurant_id_1=1&restaurant_id_2=2`, h);
  check(compareRes, { 'compare restaurants': (r) => r.status === 200 || r.status === 404 });

  // Reviews
  const reviewsRes = http.get(`${BASE_URL}/reviews/1?page=1&limit=10`, h);
  check(reviewsRes, { 'get reviews': (r) => r.status === 200 || r.status === 404 });

  // Search
  const searchRes = http.get(`${BASE_URL}/search?page=1&limit=10`, h);
  check(searchRes, { 'search': (r) => r.status === 200 || r.status === 404 });

  // Payments
  const payRes = http.get(`${BASE_URL}/payment`, h);
  check(payRes, { 'get receipts': (r) => r.status === 200 || r.status === 404 });

  // Reservations (create + cancel)
  const bookRes = http.post(
    `${BASE_URL}/reservations/1`,
    JSON.stringify({ quantity: 1, time_start: futureTimestamp() }),
    h,
  );
  check(bookRes, { 'create reservation': (r) => r.status === 201 || r.status === 400 || r.status === 404 });

  const bookingId = bookRes.json('booking_id');
  if (bookingId) {
    const cancelRes = http.del(`${BASE_URL}/reservations/${bookingId}`, null, h);
    check(cancelRes, { 'cancel reservation': (r) => r.status === 204 });
  }

  // Sponsorship
  const sponsorRes = http.get(`${BASE_URL}/sponsor/1`, h);
  check(sponsorRes, { 'get sponsor': (r) => r.status === 200 || r.status === 404 });

  // Logout
  const logoutRes = http.post(`${BASE_URL}/auth/logout`, null, h);
  check(logoutRes, { 'logout': (r) => r.status === 200 });
}
