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


def write_restaurant_csvs(
    restaurants: Iterable, restaurants_file: Path, working_hours_file: Path, categories_file: Path
) -> tuple:
    """Write restaurant data to CSV files from an iterable to keep memory usage low."""
    restaurants_file.parent.mkdir(parents=True, exist_ok=True)

    # First pass: count restaurants for progress reporting
    restaurants_list = list(restaurants)
    total_restaurants = len(restaurants_list)

    print_progress_start("Writing restaurant CSV files")
    last_pct = 0

    working_hours_count = 0
    categories_count = 0

    with (
        restaurants_file.open("w", newline="", encoding="utf-8") as restaurants_fp,
        working_hours_file.open("w", newline="", encoding="utf-8") as working_hours_fp,
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
                "media_url",
                "max_slots",
                "owner_id",
                "owner_name",
                "sponsor_tier",
            ],
        )
        working_hours_writer = csv.DictWriter(
            working_hours_fp,
            fieldnames=["restaurant_id", "time_start", "time_end"],
        )
        categories_writer = csv.DictWriter(
            categories_fp,
            fieldnames=["restaurant_id", "category"],
        )

        restaurants_writer.writeheader()
        working_hours_writer.writeheader()
        categories_writer.writeheader()

        if total_restaurants == 0:
            print_progress_end("Writing restaurant CSV files")
            return 0, 0, 0

        for index, restaurant in enumerate(restaurants_list, start=1):
            restaurants_writer.writerow(
                {
                    "id": restaurant.id,
                    "name": restaurant.name,
                    "latitude": restaurant.latitude,
                    "longitude": restaurant.longitude,
                    "address": restaurant.address,
                    "media_url": restaurant.media_url,
                    "max_slots": restaurant.max_slots,
                    "owner_id": restaurant.owner_id,
                    "owner_name": restaurant.owner_name,
                    "sponsor_tier": restaurant.sponsor_tier,
                }
            )

            if restaurant.working_hours:
                time_start, time_end = restaurant.working_hours
                working_hours_writer.writerow({"restaurant_id": restaurant.id, "time_start": time_start, "time_end": time_end})
                working_hours_count += 1

            for value in restaurant.categories:
                categories_writer.writerow({"restaurant_id": restaurant.id, "category": value})
                categories_count += 1

            last_pct = print_progress_step("Writing restaurant CSV files", index, total_restaurants, last_pct)

    if last_pct < 100:
        print_progress_end("Writing restaurant CSV files")

    return len(restaurants_list), working_hours_count, categories_count
