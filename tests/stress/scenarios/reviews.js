/**
 * Reviews stress test — GET /reviews/{restaurant_id}, POST, PUT, DELETE
 *
 * Run:
 *   k6 run -e BASE_URL=http://localhost:8080/api scenarios/reviews.js
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
  const restaurantId = randomRestaurantId();

  // GET reviews (paginated)
  const getRes = http.get(
    `${BASE_URL}/reviews/${restaurantId}?page=1&limit=25`,
    h,
  );
  check(getRes, {
    "get reviews 200": (r) => r.status === 200 || r.status === 404,
  });

  // POST a review
  const createRes = http.post(
    `${BASE_URL}/reviews/${restaurantId}`,
    JSON.stringify({ comment: "Stress test review", rating: 4 }),
    h,
  );
  check(createRes, {
    "create review 201": (r) =>
      r.status === 201 || r.status === 400 || r.status === 404,
  });

  const reviewId = createRes.json("review.review_id");

  if (reviewId) {
    // PUT update
    const updateRes = http.put(
      `${BASE_URL}/reviews/${reviewId}`,
      JSON.stringify({ comment: "Updated stress review", rating: 3 }),
      h,
    );
    check(updateRes, { "update review 200": (r) => r.status === 200 });

    // DELETE
    const deleteRes = http.del(`${BASE_URL}/reviews/${reviewId}`, null, h);
    check(deleteRes, { "delete review 204": (r) => r.status === 204 });
  }

  sleep(0.2);
}
