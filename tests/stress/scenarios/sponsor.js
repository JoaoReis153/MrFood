/**
 * Sponsor stress test — GET /sponsor/{id}, POST /sponsor/{id}
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api scenarios/sponsor.js
 */
import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, THRESHOLDS } from "../config.js";
import { getToken, authHeaders, ensureStressUser } from "../lib/auth.js";
import { randomRestaurantId } from "../lib/utils.js";

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
