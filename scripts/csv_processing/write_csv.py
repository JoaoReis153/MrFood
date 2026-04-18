import csv
from pathlib import Path
from typing import Iterable, List, Optional

from .service_seed_common import print_progress_end, print_progress_start, print_progress_step


def write_auth_csv(users: List[dict], output_file: Path) -> int:
    """Write auth users to CSV file."""
    output_file.parent.mkdir(parents=True, exist_ok=True)

    print_progress_start("Writing auth CSV")
    last_pct = 0

    with output_file.open("w", newline="", encoding="utf-8") as fp:
        writer = csv.DictWriter(fp, fieldnames=["user_id", "username", "password", "email"])
        writer.writeheader()

        total = len(users)
        for index, user in enumerate(users, start=1):
            writer.writerow(user)
            last_pct = print_progress_step("Writing auth CSV", index, total, last_pct)

    if last_pct < 100:
        print_progress_end("Writing auth CSV")

    return len(users)


def write_booking_csv(bookings: List[dict], output_file: Path) -> int:
    """Write bookings to CSV file."""
    output_file.parent.mkdir(parents=True, exist_ok=True)

    print_progress_start("Writing booking CSV")
    last_pct = 0

    with output_file.open("w", newline="", encoding="utf-8") as fp:
        writer = csv.DictWriter(
            fp,
            fieldnames=["id", "user_id", "restaurant_id", "time_start", "time_end", "people_count"],
        )
        writer.writeheader()

        total = len(bookings)
        for index, booking in enumerate(bookings, start=1):
            writer.writerow(booking)
            last_pct = print_progress_step("Writing booking CSV", index, total, last_pct)

    if last_pct < 100:
        print_progress_end("Writing booking CSV")

    return len(bookings)


def write_booking_csv_stream(bookings: Iterable[dict], output_file: Path, total: Optional[int] = None) -> int:
    """Write bookings from an iterator to keep memory usage low."""
    output_file.parent.mkdir(parents=True, exist_ok=True)

    print_progress_start("Writing booking CSV")
    last_pct = 0
    written = 0

    with output_file.open("w", newline="", encoding="utf-8") as fp:
        writer = csv.DictWriter(
            fp,
            fieldnames=["id", "user_id", "restaurant_id", "time_start", "time_end", "people_count"],
        )
        writer.writeheader()

        for written, booking in enumerate(bookings, start=1):
            writer.writerow(booking)
            if total is not None:
                last_pct = print_progress_step("Writing booking CSV", written, total, last_pct)

    if total is None:
        print_progress_end("Writing booking CSV")
    elif last_pct < 100:
        print_progress_end("Writing booking CSV")

    return written


def write_review_csv_stream(reviews: Iterable[dict], output_file: Path, total: Optional[int] = None) -> int:
    """Write reviews from an iterator to keep memory usage low."""
    output_file.parent.mkdir(parents=True, exist_ok=True)

    print_progress_start("Writing review CSV")
    last_pct = 0
    written = 0

    with output_file.open("w", newline="", encoding="utf-8") as fp:
        writer = csv.DictWriter(
            fp,
            fieldnames=["review_id", "restaurant_id", "user_id", "comment", "rating", "created_at"],
        )
        writer.writeheader()

        for written, review in enumerate(reviews, start=1):
            writer.writerow(review)
            if total is not None:
                last_pct = print_progress_step("Writing review CSV", written, total, last_pct)

    if total is None:
        print_progress_end("Writing review CSV")
    elif last_pct < 100:
        print_progress_end("Writing review CSV")

    return written


def write_restaurant_csvs(
    restaurants: Iterable, restaurants_file: Path, working_hours_file: Path, categories_file: Path
) -> tuple:
    """Write restaurant data to CSV files from an iterable to keep memory usage low."""
    restaurants_file.parent.mkdir(parents=True, exist_ok=True)

    print_progress_start("Writing restaurant CSV files")

    restaurants_count = 0
    categories_count = 0
    restaurant_ids: List[str] = []
    gplus_place_id_to_restaurant_id: dict = {}

    with (
        restaurants_file.open("w", newline="", encoding="utf-8") as restaurants_fp,
        categories_file.open("w", newline="", encoding="utf-8") as categories_fp,
    ):
        restaurants_writer = csv.DictWriter(
            restaurants_fp,
            fieldnames=[
                "id",
                "name",
                "latitude",
                "longitude",
                "address",
                "opening_time",
                "closing_time",
                "media_url",
                "max_slots",
                "owner_id",
                "owner_name",
                "sponsor_tier",
            ],
        )
        categories_writer = csv.DictWriter(
            categories_fp,
            fieldnames=["restaurant_id", "category"],
        )

        restaurants_writer.writeheader()
        categories_writer.writeheader()

        for restaurants_count, restaurant in enumerate(restaurants, start=1):
            restaurant_ids.append(str(restaurant.id))
            if restaurant.source_place_id:
                gplus_place_id_to_restaurant_id[restaurant.source_place_id] = str(restaurant.id)
            
            # Extract opening and closing times from working_hours tuple or use defaults
            opening_time = "09:00:00"
            closing_time = "17:00:00"
            if restaurant.working_hours:
                opening_time, closing_time = restaurant.working_hours
            
            restaurants_writer.writerow(
                {
                    "id": restaurant.id,
                    "name": restaurant.name,
                    "latitude": restaurant.latitude,
                    "longitude": restaurant.longitude,
                    "address": restaurant.address,
                    "opening_time": opening_time,
                    "closing_time": closing_time,
                    "media_url": restaurant.media_url,
                    "max_slots": restaurant.max_slots,
                    "owner_id": restaurant.owner_id,
                    "owner_name": restaurant.owner_name,
                    "sponsor_tier": restaurant.sponsor_tier,
                }
            )

            for value in restaurant.categories:
                categories_writer.writerow({"restaurant_id": restaurant.id, "category": value})
                categories_count += 1

    print_progress_end("Writing restaurant CSV files")

    return restaurants_count, 0, categories_count, restaurant_ids, gplus_place_id_to_restaurant_id
