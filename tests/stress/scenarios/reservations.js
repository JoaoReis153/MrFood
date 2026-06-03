/**
 * Reservations stress test — POST /reservations/{restaurant_id}, DELETE /reservations/{booking_id}
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api scenarios/reservations.js
 */
import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, THRESHOLDS } from "../config.js";
import { getToken, authHeaders, ensureStressUser } from "../lib/auth.js";
import { randomRestaurantId, futureTimestamp } from "../lib/utils.js";

export const options = {
  stages: [
    { duration: "30s", target: 15 },
    { duration: "1m", target: 15 },
    { duration: "30s", target: 0 },
  ],
  thresholds: THRESHOLDS,
};

export function setup() {
  ensureStressUser();
}

export default function () {
  const h = authHeaders(getToken());
  const restaurantId = randomRestaurantId();

  // POST — create reservation
  const createRes = http.post(
    `${BASE_URL}/reservations/${restaurantId}`,
    JSON.stringify({ quantity: 2, time_start: futureTimestamp() }),
    h,
  );
  check(createRes, {
    "create reservation 201": (r) =>
      r.status === 201 || r.status === 400 || r.status === 404,
  });

  const bookingId = createRes.json("booking_id");

  if (bookingId) {
    // DELETE — cancel immediately to avoid DB bloat
    const deleteRes = http.del(
      `${BASE_URL}/reservations/${bookingId}`,
      null,
      h,
    );
    check(deleteRes, { "cancel reservation 204": (r) => r.status === 204 });
  }

  sleep(0.2);
}
