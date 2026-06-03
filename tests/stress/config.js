// Base URL: set via -e BASE_URL=http://... or env var BASE_URL
// Defaults to local dev gateway.
export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080/api';

// Test user credentials — must exist in the target environment.
// Create with: make load-local or seed a stress user beforehand.
export const EMAIL    = __ENV.STRESS_EMAIL    || 'mrfood.api@gmail.com';
export const PASSWORD = __ENV.STRESS_PASSWORD || 'StrongPassword123';

// IDs assumed to exist in the seeded dataset.
export const RESTAURANT_IDS = [1, 2, 3, 4, 5];

export const THRESHOLDS = {
  http_req_failed:   ['rate<0.01'],          // <1% errors
  http_req_duration: ['p(95)<500'],          // 95th pct under 500ms
};
