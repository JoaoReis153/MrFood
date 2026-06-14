/**
 * Auth stress test — POST /auth/login, /auth/refresh, /auth/logout
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api scenarios/auth.js
 */
import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, THRESHOLDS } from "../config.js";
import { authHeaders, ensureStressUser } from "../lib/auth.js";

export const options = {
  stages: [
    { duration: "30s", target: 20 },
    { duration: "1m", target: 20 },
    { duration: "30s", target: 0 },
  ],
  thresholds: THRESHOLDS,
};

export function setup() {
  ensureStressUser();
}

export default function () {
  const headers = { headers: { "Content-Type": "application/json" } };

  // Login
  const loginRes = http.post(
    `${BASE_URL}/auth/login`,
    JSON.stringify({
      email: __ENV.STRESS_EMAIL || "stress@mrfood.test",
      password: __ENV.STRESS_PASSWORD || "StressTest123",
    }),
    headers,
  );
  check(loginRes, { "login 200": (r) => r.status === 200 });

  const accessToken = loginRes.json("access_token");
  const refreshToken = loginRes.json("refresh_token");

  // Refresh using the token we just received — not a shared one
  if (refreshToken) {
    const refreshRes = http.post(
      `${BASE_URL}/auth/refresh`,
      JSON.stringify({ refresh_token: refreshToken }),
      headers,
    );
    check(refreshRes, { "refresh 200": (r) => r.status === 200 });
  }

  // Logout
  if (accessToken) {
    const logoutRes = http.post(
      `${BASE_URL}/auth/logout`,
      null,
      authHeaders(accessToken),
    );
    check(logoutRes, { "logout 200": (r) => r.status === 200 });
  }

  sleep(0.2);
}
