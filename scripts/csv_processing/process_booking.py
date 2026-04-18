from datetime import datetime, timedelta
from typing import Iterator, List

def generate_bookings_stream(
    user_ids: List[str],
    restaurant_ids: List[str],
    total_bookings: int,
) -> Iterator[dict]:
    """Yield booking records without storing the full output in memory."""
    user_count = len(user_ids)
    restaurant_count = len(restaurant_ids)

    if user_count <= 0 or restaurant_count <= 0 or total_bookings <= 0:
        return

    base_date = datetime(2026, 4, 1)

    for row_idx in range(total_bookings):
        user_id = user_ids[row_idx % user_count]
        restaurant_id = restaurant_ids[(row_idx * 7) % restaurant_count]

        day_offset = row_idx % 120
        hour = 11 + (row_idx % 10)
        minute = 30 if (row_idx % 2) else 0

        time_start = base_date + timedelta(days=day_offset, hours=hour, minutes=minute)
        time_end = time_start + timedelta(minutes=90)
        # Limit people_count to 1-4 to ensure bookings don't exceed max_slots (15)
        # With multiple bookings per slot, this keeps us well under the limit
        people_count = 1 + (row_idx % 4)

        yield {
            "id": row_idx + 1,
            "user_id": user_id,
            "restaurant_id": restaurant_id,
            "time_start": time_start.strftime("%Y-%m-%d %H:%M:%S"),
            "time_end": time_end.strftime("%Y-%m-%d %H:%M:%S"),
            "people_count": people_count,
        }
