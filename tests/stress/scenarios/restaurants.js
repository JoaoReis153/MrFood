/**
 * Restaurants stress test — GET /restaurants/{id}, GET /restaurants/compare, PUT /restaurants/{id}
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api scenarios/restaurants.js
 */
import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, THRESHOLDS } from "../config.js";
import { getToken, authHeaders, ensureStressUser } from "../lib/auth.js";
import { randomRestaurantId, pick } from "../lib/utils.js";

export const options = {
  stages: [
    { duration: "30s", target: 30 },
    { duration: "1m", target: 30 },
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

  // GET /restaurants/{id}
  const getRes = http.get(`${BASE_URL}/restaurants/${id}`, h);
  check(getRes, {
    "get restaurant 200": (r) => r.status === 200 || r.status === 404,
  });

  // GET /restaurants/compare
  const id2 = randomRestaurantId();
  const compareRes = http.get(
    `${BASE_URL}/restaurants/compare?restaurant_id_1=${id}&restaurant_id_2=${id2}`,
    h,
  );
  check(compareRes, {
    "compare 200": (r) => r.status === 200 || r.status === 404,
  });

  sleep(0.2);
}
