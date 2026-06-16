/**
 * Payments stress test — GET /payment, GET /payment/{receipt_id}
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api scenarios/payments.js
 */
import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, THRESHOLDS } from "../config.js";
import { getToken, authHeaders, ensureStressUser } from "../lib/auth.js";

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

  // GET all receipts for the authenticated user
  const listRes = http.get(`${BASE_URL}/payment`, h);
  check(listRes, {
    "get receipts 200": (r) => r.status === 200 || r.status === 404,
  });

  // GET a specific receipt (try IDs 1–5; 404 is fine)
  const receiptId = Math.floor(Math.random() * 5) + 1;
  const getRes = http.get(`${BASE_URL}/payment/${receiptId}`, h);
  check(getRes, {
    "get receipt 200/404": (r) => r.status === 200 || r.status === 404,
  });

  sleep(0.2);
}
