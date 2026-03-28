# restaurant Microservice

## Quick Start

```bash
make build
make docker-build
make docker-up
```

## Endpoints
Users
 - POST /auth/register - Register a new user
 - POST /auth/login - Login user
 - POST /auth/logout - Logout user

Restaurants 
 - GET /restaurants - Search restaurants
 - GET /restaurants/{restaurantId} - Get restaurant details
 - GET /restaurants/{restaurantId}/similar - Find similar restaurants
 - GET /restaurants/compare - Compare restaurants

Reviews
 - GET /restaurants/{restaurantId}/reviews - Get reviews of a restaurant
 - POST /restaurants/{restaurantId}/reviews - Add a review for a restaurant
 - PUT /restaurants/{restaurantId}/reviews/{reviewId} - Edit a review for a restaurant
 - DELETE /restaurants/{restaurantId}/reviews/{reviewId} - Delete a review for a restaurant

Reservations
 - Post /reservations - Book a reservation
 - GET /reservations - Get available booking slots for a given hour
 - GET /reservations/minimum - Get hours with a minimum number of booking slots available

Notifications
 - GET /notifications - Get user notifications and discounts

## Development

```bash
go run cmd/main.go
make test
make lint
```
