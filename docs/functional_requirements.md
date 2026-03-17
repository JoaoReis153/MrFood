# FUNCTIONAL REQUIREMENTS
## USER CASES
### 1. REVIEW A RESTAURANT
- The system should allow authenticated users create,edit or remove a review associated with their own id of a restaurant of their choosing.
- This review will contain a rating from 1 to 5 and optionally a comment of their choosing.
- An authenticated can only have one review per restaurant.
- Each new review will also update the average rating and number of rating of each restaurant.

### 2. SEE DETAILS OF RESTAURANT
- The system should allow authenticated users to see the name, location, address, working hours, categories, avg ratings, num of ratings ,and if available media associated with restaurant.
- Optionally we can show the user if the restaurant is currently using the sponsor system or not.

### 3. SEE REVIEWS OF A RESTAURANT
- The system should allow authenticated users to see a pagination of the reviews that the selected restaurant has.
- This pagination should have a max limit defined like 30 reviews per page.
- Each review should contain the rating associated with it and the details of the user that has posted, in this case only its name.

### 4. Search for nearby restaurants
- Given the current location of the user, the system should give a pagination of the nearest restaurants.
- Optionally the user should also give some categories to filter those same restaurants.
- The user should should also be able to put a prefix of the restaurant to get matches.

### 5. Search for restaurants semantically
- Authenticated user can search for restaurant given some parameters.
- Those parameters can  be categories, virtual categories, address.
- The System should return a pagination response.

### 6 Users can see available booking slots
- Authenticated user can see the available booking slots for a given hour and restaurant.
- This response should contain number of slots available for that hour, and max number of slots avaible for that restaurant.
- Users can also search for hours that have a given number of available slots.(ex.. day x number 10 )

### 7. Users can book a reservation
- Authenticated users can book a reservation for a certain hour in a restaurant. 
- The booking must contain number of people that are going, and this number must be less or equal to the number of available slots for that hour.

### 8. Users must authenticate to use our services
- All paths, excluding the /auth are blocked to unathenticated users.
- Users must login into an account to authenticate.
- If a user doesn't have an account, they are allowed to create one by registering.

### 9. Users will get notifications if their payments go through or not.
- All users will get emails from confirmations of payments that go through.

### 10. Users can compare restaurant details
- Authenticated user









- The system should also allow the users to see the available booking slots(these booking slots should be only from 1hour)   