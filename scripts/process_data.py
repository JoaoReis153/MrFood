import argparse
from pathlib import Path
from typing import Dict, Iterable, List, Set

import pandas as pd

from csv_processing.process_auth import collect_source_user_ids, stream_auth_csv
from csv_processing.process_booking import generate_bookings_stream
from csv_processing.process_restaurant import build_restaurant_data_from_csv
from csv_processing.service_seed_common import OUTPUT_DIR, print_progress_end, print_progress_start, print_progress_step
from csv_processing.write_csv import write_booking_csv_stream, write_restaurant_csvs

SCRIPT_DIR = Path(__file__).resolve().parent
PROJECT_DIR = SCRIPT_DIR.parent
DATA_DIR = PROJECT_DIR / "data"

REQUIRED_FILES = [
    "places.csv",
    "reviews.csv",
    "users.csv",
    "tripadvisor_european_restaurants.csv",
]

DATASET_FILES = {
    "places": "places.csv",
    "reviews": "reviews.csv",
    "users": "users.csv",
    "tripadvisor": "tripadvisor_european_restaurants.csv",
}

DATASET_USECOLS = {
    "users": ["userName", "gPlusUserId"],
    "reviews": ["gPlusPlaceId", "categories"],
    "places": [
        "name",
        "gPlusPlaceId",
        "address_line1",
        "address_line2",
        "address_line3",
        "latitude",
        "longitude",
        "hours_monday",
        "hours_tuesday",
        "hours_wednesday",
        "hours_thursday",
        "hours_friday",
        "hours_saturday",
        "hours_sunday",
    ],
    "tripadvisor": [
        "restaurant_name",
        "latitude",
        "longitude",
        "address",
        "claimed",
        "top_tags",
        "cuisines",
        "meals",
        "features",
    ],
}

SERVICE_DATASET_REQUIREMENTS = {
    "auth": set(),
    "restaurant": {"places", "reviews", "tripadvisor"},
    # Booking generation depends on generated users and restaurants.
    "booking": {"places", "reviews", "tripadvisor"},
}

SERVICE_SOURCE_FILE_REQUIREMENTS = {
    "auth": {"users"},
    "restaurant": {"places", "reviews", "tripadvisor"},
    "booking": {"users", "places", "reviews", "tripadvisor"},
}

SERVICE_ORDER = ["auth", "restaurant", "booking"]


def read_csv(file_name: str, nrows: int = None, usecols: List[str] = None) -> pd.DataFrame:
    """Read a CSV file from the data directory."""
    path = DATA_DIR / file_name

    def read_with_engine(selected_usecols: List[str], python_engine: bool) -> pd.DataFrame:
        dtype_candidates = {
            "gPlusPlaceId": "string",
            "gPlusUserId": "string",
        }
        if selected_usecols is None:
            selected_dtype = dtype_candidates
        else:
            selected_dtype = {k: v for k, v in dtype_candidates.items() if k in selected_usecols}

        kwargs = {
            "nrows": nrows,
            "dtype": selected_dtype if selected_dtype else None,
        }
        if selected_usecols is not None:
            kwargs["usecols"] = selected_usecols

        if python_engine:
            kwargs["engine"] = "python"
            kwargs["on_bad_lines"] = "skip"

        return pd.read_csv(path, **kwargs)

    print(f"  Loading {file_name}...", end="", flush=True)

    usecols_candidates = [usecols] if usecols is not None else [None]
    if usecols is not None:
        usecols_candidates.append(None)

    last_error = None
    df = None

    for selected_usecols in usecols_candidates:
        try:
            try:
                df = read_with_engine(selected_usecols, python_engine=False)
            except Exception:
                df = read_with_engine(selected_usecols, python_engine=True)

            if usecols is not None and selected_usecols is None:
                print(" (fallback to full columns)", end="", flush=True)
            break
        except ValueError as exc:
            last_error = exc
            # If expected columns don't match, retry once with full columns.
            if selected_usecols is not None and "Usecols do not match columns" in str(exc):
                continue
            raise

    if df is None and last_error is not None:
        raise last_error

    print(f" ✓ ({len(df):,} rows)")
    return df


def load_datasets(dataset_keys: Iterable[str], nrows: int = None) -> Dict[str, pd.DataFrame]:
    """Load only the requested CSV datasets."""
    requested = sorted(set(dataset_keys))
    print_progress_start("Loading source data")
    datasets = {
        key: read_csv(
            DATASET_FILES[key],
            nrows=nrows,
            usecols=DATASET_USECOLS.get(key),
        )
        for key in requested
    }
    print_progress_end("Loading source data")
    return datasets


def verify_files(dataset_keys: Iterable[str]) -> bool:
    """Verify that required data files exist for selected services."""
    missing = []
    required_files = [DATASET_FILES[key] for key in sorted(set(dataset_keys))]
    for file_name in required_files:
        if not (DATA_DIR / file_name).exists():
            missing.append(file_name)

    if missing:
        print("Missing files in data directory:")
        for file_name in missing:
            print(f" - {file_name}")
        return False

    return True


def normalize_services(requested_services: Iterable[str]) -> List[str]:
    """Normalize and order requested services."""
    requested = set(requested_services)
    if "all" in requested:
        return SERVICE_ORDER.copy()
    return [service for service in SERVICE_ORDER if service in requested]


def required_datasets_for_services(services: Iterable[str]) -> Set[str]:
    """Return the set of source datasets required by selected services."""
    required: Set[str] = set()
    for service in services:
        required.update(SERVICE_DATASET_REQUIREMENTS[service])
    return required


def required_source_files_for_services(services: Iterable[str]) -> Set[str]:
    """Return all required source files for selected services."""
    required: Set[str] = set()
    for service in services:
        required.update(SERVICE_SOURCE_FILE_REQUIREMENTS[service])
    return required


def parse_args() -> argparse.Namespace:
    """Parse command-line options."""
    parser = argparse.ArgumentParser(
        description="Generate processed CSV seed data for selected MrFood services."
    )
    parser.add_argument(
        "-s",
        "--services",
        nargs="+",
        choices=["all", "auth", "restaurant", "booking"],
        default=["all"],
        help="Services to generate: auth restaurant booking (default: all).",
    )
    parser.add_argument(
        "-n",
        "--rows",
        type=int,
        default=200,
        help="Limit number of input rows read per source CSV (default: 200).",
    )
    parser.add_argument(
        "--full",
        action="store_true",
        help="Process full dataset (overrides --rows limit).",
    )
    parser.add_argument(
        "--max-bookings",
        type=int,
        default=None,
        help="Cap total bookings generated (default: bounded automatically).",
    )
    return parser.parse_args()


def count_users(file_name: str, nrows: int = None) -> int:
    """Count source users rows without loading file into memory."""
    path = DATA_DIR / file_name
    count = 0

    print(f"  Counting rows in {file_name}...", end="", flush=True)
    with path.open("r", newline="", encoding="utf-8") as fp:
        next(fp, None)
        for count, _ in enumerate(fp, start=1):
            if nrows is not None and count >= nrows:
                break
    print(f" ✓ ({count:,} rows)")
    return count


def generate_csvs(selected_services: Iterable[str], rows: int = None, max_bookings: int = None):
    """Generate selected service CSV seed files."""
    if rows is not None and rows <= 0:
        raise ValueError("--rows must be greater than 0")

    services = normalize_services(selected_services)
    if not services:
        print("No services selected. Nothing to generate.")
        return

    source_file_keys = required_source_files_for_services(services)
    if not verify_files(source_file_keys):
        return

    dataset_keys = required_datasets_for_services(services)

    if services == ["auth"]:
        print("\n✓ Processing auth users")
        stream_auth_csv(
            DATA_DIR / DATASET_FILES["users"],
            OUTPUT_DIR / "auth" / "app_user.csv",
            nrows=rows,
        )
        print("\n✓ CSV generation completed for: auth")
        return

    # For restaurant and booking, use streaming CSV directly to avoid loading DataFrames
    user_ids: List[str] = []
    restaurant_ids: List[str] = []
    restaurant_count = 0

    if "auth" in services or "booking" in services:
        if "auth" in services:
            print("\n✓ Processing auth users")
            stream_auth_csv(
                DATA_DIR / DATASET_FILES["users"],
                OUTPUT_DIR / "auth" / "app_user.csv",
                nrows=rows,
            )
        if "booking" in services:
            print("\n✓ Collecting source user IDs for bookings")
            user_ids = collect_source_user_ids(
                DATA_DIR / DATASET_FILES["users"],
                nrows=rows,
            )

    if "restaurant" in services or "booking" in services:
        print("\n✓ Processing restaurants")
        # Stream directly from CSV files instead of loading DataFrames
        restaurants_stream = build_restaurant_data_from_csv(
            DATA_DIR / DATASET_FILES["places"],
            DATA_DIR / DATASET_FILES["reviews"],
            DATA_DIR / DATASET_FILES["tripadvisor"],
            nrows=rows,
        )
        if "restaurant" in services:
            restaurant_count, _, _, restaurant_ids = write_restaurant_csvs(
                restaurants_stream,
                OUTPUT_DIR / "restaurant" / "restaurants.csv",
                OUTPUT_DIR / "restaurant" / "restaurant_working_hours.csv",
                OUTPUT_DIR / "restaurant" / "restaurant_categories.csv",
            )
        elif "booking" in services:
            # Only booking requested: collect restaurant IDs from stream for booking generation
            restaurant_ids = [str(restaurant.id) for restaurant in restaurants_stream]
            restaurant_count = len(restaurant_ids)

    if "booking" in services:
        print("\n✓ Processing bookings")
        if not user_ids:
            user_ids = collect_source_user_ids(DATA_DIR / DATASET_FILES["users"], nrows=rows)

        if not restaurant_ids:
            print("\n✓ Collecting restaurant IDs for bookings")
            restaurants_stream = build_restaurant_data_from_csv(
                DATA_DIR / DATASET_FILES["places"],
                DATA_DIR / DATASET_FILES["reviews"],
                DATA_DIR / DATASET_FILES["tripadvisor"],
                nrows=rows,
            )
            restaurant_ids = [str(restaurant.id) for restaurant in restaurants_stream]
            restaurant_count = len(restaurant_ids)

        total_bookings = len(user_ids) * restaurant_count
        booking_stream = generate_bookings_stream(
            user_ids=user_ids,
            restaurant_ids=restaurant_ids,
            total_bookings=total_bookings,
        )
        write_booking_csv_stream(booking_stream, OUTPUT_DIR / "booking" / "booking.csv", total=total_bookings)

    service_list = ", ".join(services)
    print(f"\n✓ CSV generation completed for: {service_list}")


if __name__ == "__main__":
    args = parse_args()
    rows = None if args.full else args.rows
    generate_csvs(args.services, rows=rows, max_bookings=args.max_bookings)

