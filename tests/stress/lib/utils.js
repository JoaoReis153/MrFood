import { RESTAURANT_IDS } from '../config.js';

/** Pick a random element from an array. */
export function pick(arr) {
  return arr[Math.floor(Math.random() * arr.length)];
}

/** Pick a random restaurant ID from the seeded set. */
export function randomRestaurantId() {
  return pick(RESTAURANT_IDS);
}

/** Future ISO timestamp offset by `offsetMs` from now. */
export function futureTimestamp(offsetMs = 7 * 24 * 60 * 60 * 1000) {
  return new Date(Date.now() + offsetMs).toISOString();
}
