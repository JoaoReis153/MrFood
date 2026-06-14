/**
 * Full stress test — runs all domains concurrently using k6 scenarios.
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api full.js
 *
 * Override load:
 *   k6 run -e BASE_URL=... -e VUS=50 -e DURATION=5m full.js
 */
import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, EMAIL, PASSWORD, THRESHOLDS } from "./config.js";
import { getToken, authHeaders, ensureStressUser } from "./lib/auth.js";
import { randomRestaurantId, futureTimestamp, pick } from "./lib/utils.js";

const VUS = parseInt(__ENV.VUS || "20");
const DURATION = __ENV.DURATION || "10m";

export const options = {
  scenarios: {
    auth: {
      executor: "constant-vus",
      exec: "authScenario",
      vus: Math.max(1, Math.floor(VUS * 0.15)),
      duration: DURATION,
      startTime: "0s",
    },
    restaurants: {
      executor: "constant-vus",
      exec: "restaurantsScenario",
      vus: Math.max(1, Math.floor(VUS * 0.25)),
      duration: DURATION,
      startTime: "5s",
    },
    search: {
      executor: "constant-vus",
      exec: "searchScenario",
      vus: Math.max(1, Math.floor(VUS * 0.3)),
      duration: DURATION,
      startTime: "5s",
    },
    reviews: {
      executor: "constant-vus",
      exec: "reviewsScenario",
      vus: Math.max(1, Math.floor(VUS * 0.15)),
      duration: DURATION,
      startTime: "10s",
    },
    reservations: {
      executor: "constant-vus",
      exec: "reservationsScenario",
      vus: Math.max(1, Math.floor(VUS * 0.1)),
      duration: DURATION,
      startTime: "10s",
    },
    payments: {
      executor: "constant-vus",
      exec: "paymentsScenario",
      vus: Math.max(1, Math.floor(VUS * 0.05)),
      duration: DURATION,
      startTime: "10s",
    },
    sponsor: {
      executor: "constant-vus",
      exec: "sponsorScenario",
      vus: Math.max(1, Math.floor(VUS * 0.05)),
      duration: DURATION,
      startTime: "10s",
    },
  },
  thresholds: {
    ...THRESHOLDS,
    "http_req_duration{scenario:search}": ["p(95)<800"],
    "http_req_duration{scenario:restaurants}": ["p(95)<500"],
    "http_req_duration{scenario:auth}": ["p(95)<300"],
  },
};

export function setup() {
  ensureStressUser();
}

// ── Auth ────────────────────────────────────────────────────────────────────

export function authScenario() {
  const headers = { headers: { "Content-Type": "application/json" } };

  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({ email: EMAIL, password: PASSWORD }),
    headers,
  );
  check(loginRes, { "[auth] login 200": (r) => r.status === 200 });

  const refreshToken = loginRes.json("refresh_token");
  if (refreshToken) {
    const refreshRes = http.post(
      `${BASE_URL}/auth/refresh`,
      JSON.stringify({ refresh_token: refreshToken }),
      headers,
    );
    check(refreshRes, { "[auth] refresh 200": (r) => r.status === 200 });
  }

  sleep(0.2);
}

// ── Restaurants ──────────────────────────────────────────────────────────────

export function restaurantsScenario() {
  const h = authHeaders(getToken());
  const id = randomRestaurantId();

  const getRes = http.get(`${BASE_URL}/restaurants/${id}`, h);
  check(getRes, {
    "[restaurants] get 200": (r) => r.status === 200 || r.status === 404,
  });

  const id2 = randomRestaurantId();
  const cmpRes = http.get(
    `${BASE_URL}/restaurants/compare?restaurant_id_1=${id}&restaurant_id_2=${id2}`,
    h,
  );
  check(cmpRes, {
    "[restaurants] compare 200": (r) => r.status === 200 || r.status === 404,
  });

  sleep(0.2);
}

// ── Search ───────────────────────────────────────────────────────────────────

const SEARCH_QUERIES = [
  "?page=1&limit=25",
  "?category=Italian&limit=10",
  "?category=Portuguese&page=1&limit=25",
  "?name_suffix=Sobral&limit=10",
  "?latitude=38.7169&longitude=-9.1399&radius_meters=2000",
  "?latitude=40.7128&longitude=-74.0060&radius_meters=5000",
];

export function searchScenario() {
  const h = authHeaders(getToken());
  const res = http.get(`${BASE_URL}/search${pick(SEARCH_QUERIES)}`, h);
  check(res, { "[search] 200": (r) => r.status === 200 || r.status === 404 });
  sleep(0.2);
}

// ── Reviews ──────────────────────────────────────────────────────────────────

export function reviewsScenario() {
  const h = authHeaders(getToken());
  const restaurantId = randomRestaurantId();

  const getRes = http.get(
    `${BASE_URL}/reviews/${restaurantId}?page=1&limit=25`,
    h,
  );
  check(getRes, {
    "[reviews] get 200": (r) => r.status === 200 || r.status === 404,
  });

  const createRes = http.post(
    `${BASE_URL}/reviews/${restaurantId}`,
    JSON.stringify({ comment: "Stress test review", rating: 4 }),
    h,
  );
  check(createRes, {
    "[reviews] create 201": (r) =>
      r.status === 201 ||
      r.status === 400 ||
      r.status === 404 ||
      r.status === 409,
  });

  const reviewId = createRes.json("review.review_id");
  if (reviewId) {
    const delRes = http.del(`${BASE_URL}/reviews/${reviewId}`, null, h);
    check(delRes, { "[reviews] delete 204": (r) => r.status === 204 });
  }

  sleep(0.2);
}

// ── Reservations ──────────────────────────────────────────────────────────────

export function reservationsScenario() {
  const h = authHeaders(getToken());
  const restaurantId = randomRestaurantId();

  const createRes = http.post(
    `${BASE_URL}/reservations/${restaurantId}`,
    JSON.stringify({ quantity: 2, time_start: futureTimestamp() }),
    h,
  );
  check(createRes, {
    "[reservations] create 201": (r) =>
      r.status === 201 ||
      r.status === 400 ||
      r.status === 404 ||
      r.status === 409,
  });

  const bookingId = createRes.json("booking_id");
  if (bookingId) {
    const delRes = http.del(`${BASE_URL}/reservations/${bookingId}`, null, h);
    check(delRes, { "[reservations] cancel 204": (r) => r.status === 204 });
  }

  sleep(0.2);
}

// ── Payments ──────────────────────────────────────────────────────────────────

export function paymentsScenario() {
  const h = authHeaders(getToken());

  const listRes = http.get(`${BASE_URL}/payment`, h);
  check(listRes, {
    "[payments] list 200": (r) => r.status === 200 || r.status === 404,
  });

  const receiptId = Math.floor(Math.random() * 5) + 1;
  const getRes = http.get(`${BASE_URL}/payment/${receiptId}`, h);
  check(getRes, {
    "[payments] get 200/404": (r) => r.status === 200 || r.status === 404,
  });

  sleep(0.2);
}

// ── Sponsor ───────────────────────────────────────────────────────────────────

export function sponsorScenario() {
  const h = authHeaders(getToken());
  const id = randomRestaurantId();

  const getRes = http.get(`${BASE_URL}/sponsor/${id}`, h);
  check(getRes, {
    "[sponsor] get 200/404": (r) => r.status === 200 || r.status === 404,
  });

  const sponsorRes = http.post(
    `${BASE_URL}/sponsor/${id}`,
    JSON.stringify({ tier: 1 }),
    h,
  );
  check(sponsorRes, {
    "[sponsor] create 200/404/409": (r) =>
      r.status === 200 || r.status === 404 || r.status === 409,
  });

  sleep(0.2);
}
