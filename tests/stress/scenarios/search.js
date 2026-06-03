/**
 * Search stress test — GET /search with various query combinations
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api scenarios/search.js
 */
import http from "k6/http";
import { check, sleep } from "k6";
import { BASE_URL, THRESHOLDS } from "../config.js";
import { getToken, authHeaders, ensureStressUser } from "../lib/auth.js";
import { pick } from "../lib/utils.js";

export const options = {
  stages: [
    { duration: "30s", target: 40 },
    { duration: "2m", target: 40 },
    { duration: "30s", target: 0 },
  ],
  thresholds: {
    ...THRESHOLDS,
    http_req_duration: ["p(95)<800"], // search allowed a bit more headroom
  },
};

const QUERIES = [
  "?page=1&limit=25",
  "?category=Italian&limit=10",
  "?category=Portuguese&page=1&limit=25",
  "?name_suffix=Sobral&limit=10",
  "?latitude=38.7169&longitude=-9.1399&radius_meters=2000",
  "?latitude=40.7128&longitude=-74.0060&radius_meters=5000",
  "?full_name=Doce Sobral",
];

export function setup() {
  ensureStressUser();
}

export default function () {
  const h = authHeaders(getToken());
  const query = pick(QUERIES);

  const res = http.get(`${BASE_URL}/search${query}`, h);
  check(res, { "search 200": (r) => r.status === 200 || r.status === 404 });

  sleep(0.2);
}
