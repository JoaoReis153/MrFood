from datetime import datetime, timedelta
from typing import Iterator, Optional

DEFAULT_MAX_BOOKINGS = 250000


def resolve_max_bookings(user_count: int, restaurant_count: int, requested_max: Optional[int] = None) -> int:
    """Resolve booking count while keeping generation bounded for large datasets."""
    if user_count <= 0 or restaurant_count <= 0:
        return 0

    theoretical = user_count * restaurant_count
    if requested_max is not None:
        return min(requested_max, theoretical)
    return min(DEFAULT_MAX_BOOKINGS, theoretical)


def generate_bookings_stream(
    user_count: int,
    restaurant_count: int,
    total_bookings: int,
) -> Iterator[dict]:
    """Yield booking records without storing the full output in memory."""
    if user_count <= 0 or restaurant_count <= 0 or total_bookings <= 0:
        return

    base_date = datetime(2026, 4, 1)

    for row_idx in range(total_bookings):
        user_id = (row_idx % user_count) + 1
        restaurant_id = ((row_idx * 7) % restaurant_count) + 1

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
