from __future__ import annotations

import csv
import re
from dataclasses import dataclass
from datetime import datetime
from pathlib import Path
from typing import Dict, Iterable, List, Optional

import pandas as pd

from .service_seed_common import (
    clean_id,
    clean_text,
    normalize_name,
    print_progress_end,
    print_progress_start,
    print_progress_step,
)


def stream_reviews_csv(file_path: Path, nrows: Optional[int] = None) -> Iterable[dict]:
    """Stream review records from CSV without loading into memory."""
    count = 0
    with file_path.open("r", newline="", encoding="utf-8") as fp:
        reader = csv.DictReader(fp)
        for row in reader:
            if nrows is not None and count >= nrows:
                break
            yield row
            count += 1


def stream_places_csv(file_path: Path, nrows: Optional[int] = None) -> Iterable[dict]:
    """Stream place records from CSV without loading into memory."""
    count = 0
    with file_path.open("r", newline="", encoding="utf-8") as fp:
        reader = csv.DictReader(fp)
        for row in reader:
            if nrows is not None and count >= nrows:
                break
            yield row
            count += 1


def stream_tripadvisor_csv(file_path: Path, nrows: Optional[int] = None) -> Iterable[dict]:
    """Stream tripadvisor records from CSV without loading into memory."""
    count = 0
    with file_path.open("r", newline="", encoding="utf-8") as fp:
        reader = csv.DictReader(fp)
        for row in reader:
            if nrows is not None and count >= nrows:
                break
            yield row
            count += 1


@dataclass
class RestaurantRecord:
    id: str
    name: str
    latitude: float
    longitude: float
    address: str
    media_url: str
    max_slots: int
    owner_id: int
    owner_name: str
    sponsor_tier: int
    source_place_id: Optional[str]
    working_hours: List[str]
    categories: List[str]


def parse_time_token(token: object):
    """Parse a time token from various formats into a time object."""
    token_text = clean_text(token).lower()
    if not token_text:
        return None

    for fmt in ["%I:%M %p", "%I %p", "%H:%M", "%H"]:
        try:
            return datetime.strptime(token_text, fmt).time()
        except ValueError:
            continue
    return None


def parse_hours_range(hours_str: object):
    """Parse opening/closing hours from a range string (e.g., '09:00 -- 17:00')."""
    hours_text = clean_text(hours_str)
    if not hours_text or "--" not in hours_text:
        return None, None

    left, right = [part.strip() for part in hours_text.split("--", 1)]
    open_t = parse_time_token(left)
    close_t = parse_time_token(right)
    return open_t, close_t


def compose_address(parts: List[object]) -> str:
    """Combine address parts into a single string."""
    filtered = [clean_text(p) for p in parts if clean_text(p)]
    return ", ".join(filtered)[:100]


def parse_coordinate(value: object) -> Optional[float]:
    """Parse a coordinate value; return None when empty or invalid."""
    text = clean_text(value)
    if not text:
        return None
    try:
        return float(text)
    except (TypeError, ValueError):
        return None


def extract_categories(raw_value: object) -> List[str]:
    """Extract unique categories from a delimited string."""
    text = clean_text(raw_value)
    if not text:
        return []

    pieces = re.split(r"\s*[;,|/]\s*|\s+-\s+", text)
    categories: List[str] = []
    seen: set[str] = set()

    for part in pieces:
        category = clean_text(part, 64)
        key = category.lower()
        if not category or key in seen:
            continue
        seen.add(key)
        categories.append(category)

    return categories


def build_review_categories_map(reviews_stream: Iterable[dict]) -> Dict[str, set[str]]:
    """Index review categories by place ID from streaming review records."""
    categories_map: Dict[str, set[str]] = {}
    
    print_progress_start("Indexing review categories")
    last_pct = 0
    index = 0
    
    # Convert to list temporarily to get count for progress
    reviews_list = list(reviews_stream)
    total_reviews = len(reviews_list)

    for index, row in enumerate(reviews_list, start=1):
        place_id = clean_id(row.get("gPlusPlaceId"))
        if not place_id:
            last_pct = print_progress_step("Indexing review categories", index, total_reviews, last_pct)
            continue

        categories = extract_categories(row.get("categories"))
        if not categories:
            last_pct = print_progress_step("Indexing review categories", index, total_reviews, last_pct)
            continue

        bucket = categories_map.setdefault(place_id, set())
        for category in categories:
            bucket.add(category)

        last_pct = print_progress_step("Indexing review categories", index, total_reviews, last_pct)

    if last_pct < 100:
        print_progress_end("Indexing review categories")

    return categories_map


def build_place_working_hours(place_row: pd.Series) -> tuple:
    """Extract valid working hours from place data."""
    day_names = [
        "monday",
        "tuesday",
        "wednesday",
        "thursday",
        "friday",
        "saturday",
        "sunday",
    ]

    # Find the first day with valid hours where close_time > open_time (no midnight crossing)
    for day_name in day_names:
        open_t, close_t = parse_hours_range(place_row.get(f"hours_{day_name}"))
        if open_t is not None and close_t is not None and open_t < close_t:
            open_str = open_t.strftime("%H:%M:%S")
            close_str = close_t.strftime("%H:%M:%S")
            return (open_str, close_str)

    # Return None if no valid hours found
    return None


def build_place_working_hours_from_dict(place_dict: dict) -> tuple:
    """Extract valid working hours from place dict record."""
    day_names = [
        "monday",
        "tuesday",
        "wednesday",
        "thursday",
        "friday",
        "saturday",
        "sunday",
    ]

    # Find the first day with valid hours where close_time > open_time (no midnight crossing)
    for day_name in day_names:
        open_t, close_t = parse_hours_range(place_dict.get(f"hours_{day_name}"))
        if open_t is not None and close_t is not None and open_t < close_t:
            open_str = open_t.strftime("%H:%M:%S")
            close_str = close_t.strftime("%H:%M:%S")
            return (open_str, close_str)

    # Return None if no valid hours found
    return None


def build_restaurants_stream(
    places_stream: Iterable[dict],
    reviews_stream: Iterable[dict],
    tripadvisor_stream: Iterable[dict],
    max_restaurants: Optional[int],
):
    """Generate restaurants one at a time from streaming sources to keep memory usage constant."""
    seen_names: set[str] = set()
    review_categories = build_review_categories_map(reviews_stream)
    review_place_ids = set(review_categories.keys())

    next_fallback_restaurant_id = 1
    total_restaurants = 0

    # Process places with reviews first (prioritized), then without
    prioritized_places = []
    secondary_places = []
    
    print_progress_start("Preparing place priority list")
    last_pct = 0
    places_count = 0

    for places_count, place_row in enumerate(places_stream, start=1):
        row_place_id = clean_id(place_row.get("gPlusPlaceId"))
        if row_place_id and row_place_id in review_place_ids:
            prioritized_places.append(place_row)
        else:
            secondary_places.append(place_row)
        last_pct = print_progress_step("Preparing place priority list", places_count, places_count, last_pct)

    if last_pct < 100:
        print_progress_end("Preparing place priority list")

    # Combine prioritized and secondary
    all_places = prioritized_places + secondary_places
    place_limit = len(all_places) if max_restaurants is None else min(max_restaurants, len(all_places))
    
    print_progress_start("Building restaurants from places")
    last_pct = 0

    for place_index, place_row in enumerate(all_places, start=1):
        if max_restaurants is not None and total_restaurants >= max_restaurants:
            break

        name = clean_text(place_row.get("name"), 100)
        normalized = normalize_name(name)
        if not name or normalized in seen_names:
            continue

        latitude = parse_coordinate(place_row.get("latitude"))
        longitude = parse_coordinate(place_row.get("longitude"))
        if latitude is None or longitude is None:
            continue

        gplus_place_id = clean_id(place_row.get("gPlusPlaceId"))
        categories = sorted(list(review_categories.get(gplus_place_id, set())))

        restaurant_id = gplus_place_id or str(next_fallback_restaurant_id)
        if not gplus_place_id:
            next_fallback_restaurant_id += 1

        yield RestaurantRecord(
            id=restaurant_id,
            name=name,
            latitude=latitude,
            longitude=longitude,
            address=compose_address([
                place_row.get("address_line1"),
                place_row.get("address_line2"),
                place_row.get("address_line3"),
            ]),
            media_url="",
            max_slots=50,
            owner_id=1,
            owner_name="admin",
            sponsor_tier=0,
            source_place_id=gplus_place_id or None,
            working_hours=build_place_working_hours_from_dict(place_row),
            categories=categories,
        )
        seen_names.add(normalized)
        total_restaurants += 1
        last_pct = print_progress_step("Building restaurants from places", total_restaurants, place_limit, last_pct)

    if last_pct < 100:
        print_progress_end("Building restaurants from places")

    # Process TripAdvisor data
    tripadvisor_limit = 1000000  # Large limit, will stop at max_restaurants
    if max_restaurants is not None:
        tripadvisor_limit = max(0, max_restaurants - total_restaurants)

    print_progress_start("Building restaurants from TripAdvisor")
    last_pct = 0
    added_tripadvisor = 0

    for tripadvisor_index, tripadvisor_row in enumerate(tripadvisor_stream, start=1):
        if max_restaurants is not None and total_restaurants >= max_restaurants:
            break

        name = clean_text(tripadvisor_row.get("restaurant_name"), 100)
        normalized = normalize_name(name)
        if not name or normalized in seen_names:
            continue

        latitude = parse_coordinate(tripadvisor_row.get("latitude"))
        longitude = parse_coordinate(tripadvisor_row.get("longitude"))
        if latitude is None or longitude is None:
            continue

        categories: List[str] = []
        for field in ["top_tags", "cuisines", "meals", "features"]:
            categories.extend(extract_categories(tripadvisor_row.get(field)))

        unique_categories: List[str] = []
        seen_categories: set[str] = set()
        for category in categories:
            key = category.lower()
            if key in seen_categories:
                continue
            seen_categories.add(key)
            unique_categories.append(category)

        claimed = clean_text(tripadvisor_row.get("claimed")).lower()
        sponsor_tier = 1 if claimed == "claimed" else 0

        restaurant_id = str(next_fallback_restaurant_id)
        next_fallback_restaurant_id += 1

        yield RestaurantRecord(
            id=restaurant_id,
            name=name,
            latitude=latitude,
            longitude=longitude,
            address=clean_text(tripadvisor_row.get("address"), 100),
            media_url="",
            max_slots=15,
            owner_id=1,
            owner_name="admin",
            sponsor_tier=sponsor_tier,
            source_place_id=None,
            working_hours=[],
            categories=unique_categories[:10],
        )
        seen_names.add(normalized)
        total_restaurants += 1
        added_tripadvisor += 1
        last_pct = print_progress_step(
            "Building restaurants from TripAdvisor",
            added_tripadvisor,
            tripadvisor_limit,
            last_pct,
        )

    if last_pct < 100:
        print_progress_end("Building restaurants from TripAdvisor")


def build_restaurants(
    places_df: pd.DataFrame,
    reviews_df: pd.DataFrame,
    tripadvisor_df: pd.DataFrame,
    max_restaurants: Optional[int],
) -> List[RestaurantRecord]:
    """Build restaurants (legacy list-based method for compatibility)."""
    # Convert DataFrames to dict streams for unified processing
    places_stream = (row.to_dict() for _, row in places_df.iterrows())
    reviews_stream = (row.to_dict() for _, row in reviews_df.iterrows())
    tripadvisor_stream = (row.to_dict() for _, row in tripadvisor_df.iterrows())
    
    return list(build_restaurants_stream(places_stream, reviews_stream, tripadvisor_stream, max_restaurants))


def build_restaurant_data_from_csv(places_path: Path, reviews_path: Path, tripadvisor_path: Path, nrows: Optional[int] = None) -> Iterable:
    """Build restaurant records directly from CSV files (streaming, low memory)."""
    places_stream = stream_places_csv(places_path, nrows)
    reviews_stream = stream_reviews_csv(reviews_path, nrows)
    tripadvisor_stream = stream_tripadvisor_csv(tripadvisor_path, nrows)
    
    return build_restaurants_stream(
        places_stream=places_stream,
        reviews_stream=reviews_stream,
        tripadvisor_stream=tripadvisor_stream,
        max_restaurants=None,
    )


def build_restaurant_data(places_df: pd.DataFrame, reviews_df: pd.DataFrame, tripadvisor_df: pd.DataFrame) -> Iterable:
    """Build restaurant records from places, reviews, and tripadvisor data (streaming)."""
    places_stream = (row.to_dict() for _, row in places_df.iterrows())
    reviews_stream = (row.to_dict() for _, row in reviews_df.iterrows())
    tripadvisor_stream = (row.to_dict() for _, row in tripadvisor_df.iterrows())
    
    return build_restaurants_stream(
        places_stream=places_stream,
        reviews_stream=reviews_stream,
        tripadvisor_stream=tripadvisor_stream,
        max_restaurants=None,
    )
