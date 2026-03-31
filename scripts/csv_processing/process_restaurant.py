from __future__ import annotations

import re
from dataclasses import dataclass
from datetime import datetime
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


@dataclass
class RestaurantRecord:
    id: int
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


def build_review_categories_map(reviews_df: pd.DataFrame) -> Dict[str, set[str]]:
    """Index review categories by place ID for quick lookup."""
    categories_map: Dict[str, set[str]] = {}
    total_reviews = len(reviews_df)
    print_progress_start("Indexing review categories")
    last_pct = 0

    for index, row in enumerate(reviews_df.itertuples(index=False), start=1):
        place_id = clean_id(getattr(row, "gPlusPlaceId", None))
        if not place_id:
            last_pct = print_progress_step("Indexing review categories", index, total_reviews, last_pct)
            continue

        categories = extract_categories(getattr(row, "categories", None))
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


def build_restaurants_stream(
    places_df: pd.DataFrame,
    reviews_df: pd.DataFrame,
    tripadvisor_df: pd.DataFrame,
    max_restaurants: Optional[int],
):
    """Generate restaurants one at a time to keep memory usage constant."""
    seen_names: set[str] = set()
    review_categories = build_review_categories_map(reviews_df)
    review_place_ids = {clean_id(v) for v in reviews_df.get("gPlusPlaceId", pd.Series(dtype=str)).tolist() if clean_id(v)}

    next_id = 1
    total_restaurants = 0

    # Process places with reviews first (prioritized), then without
    prioritized_indices = []
    secondary_indices = []
    total_places = len(places_df)
    print_progress_start("Preparing place priority list")
    last_pct = 0

    for index in range(total_places):
        row_place_id = clean_id(places_df.iloc[index].get("gPlusPlaceId"))
        if row_place_id and row_place_id in review_place_ids:
            prioritized_indices.append(index)
        else:
            secondary_indices.append(index)
        last_pct = print_progress_step("Preparing place priority list", index + 1, total_places, last_pct)

    if last_pct < 100:
        print_progress_end("Preparing place priority list")

    # Process prioritized places
    place_limit = len(places_df) if max_restaurants is None else min(max_restaurants, len(places_df))
    print_progress_start("Building restaurants from places")
    last_pct = 0

    for place_index in prioritized_indices:
        if max_restaurants is not None and total_restaurants >= max_restaurants:
            break

        row = places_df.iloc[place_index]
        name = clean_text(row.get("name"), 100)
        normalized = normalize_name(name)
        if not name or normalized in seen_names:
            continue

        if pd.isna(row.get("latitude")) or pd.isna(row.get("longitude")):
            continue

        gplus_place_id = clean_id(row.get("gPlusPlaceId"))
        categories = sorted(list(review_categories.get(gplus_place_id, set())))

        yield RestaurantRecord(
            id=next_id,
            name=name,
            latitude=float(row.get("latitude")),
            longitude=float(row.get("longitude")),
            address=compose_address([
                row.get("address_line1"),
                row.get("address_line2"),
                row.get("address_line3"),
            ]),
            media_url="",
            max_slots=50,
            owner_id=1,
            owner_name="admin",
            sponsor_tier=0,
            source_place_id=gplus_place_id or None,
            working_hours=build_place_working_hours(row),
            categories=categories,
        )
        seen_names.add(normalized)
        next_id += 1
        total_restaurants += 1
        last_pct = print_progress_step("Building restaurants from places", total_restaurants, place_limit, last_pct)

    # Process remaining places
    for place_index in secondary_indices:
        if max_restaurants is not None and total_restaurants >= max_restaurants:
            break

        row = places_df.iloc[place_index]
        name = clean_text(row.get("name"), 100)
        normalized = normalize_name(name)
        if not name or normalized in seen_names:
            continue

        if pd.isna(row.get("latitude")) or pd.isna(row.get("longitude")):
            continue

        gplus_place_id = clean_id(row.get("gPlusPlaceId"))
        categories = sorted(list(review_categories.get(gplus_place_id, set())))

        yield RestaurantRecord(
            id=next_id,
            name=name,
            latitude=float(row.get("latitude")),
            longitude=float(row.get("longitude")),
            address=compose_address([
                row.get("address_line1"),
                row.get("address_line2"),
                row.get("address_line3"),
            ]),
            media_url="",
            max_slots=50,
            owner_id=1,
            owner_name="admin",
            sponsor_tier=0,
            source_place_id=gplus_place_id or None,
            working_hours=build_place_working_hours(row),
            categories=categories,
        )
        seen_names.add(normalized)
        next_id += 1
        total_restaurants += 1
        last_pct = print_progress_step("Building restaurants from places", total_restaurants, place_limit, last_pct)

    if last_pct < 100:
        print_progress_end("Building restaurants from places")

    # Process TripAdvisor data
    tripadvisor_limit = len(tripadvisor_df)
    if max_restaurants is not None:
        tripadvisor_limit = max(0, max_restaurants - total_restaurants)

    print_progress_start("Building restaurants from TripAdvisor")
    last_pct = 0
    added_tripadvisor = 0

    for tripadvisor_index in range(len(tripadvisor_df)):
        if max_restaurants is not None and total_restaurants >= max_restaurants:
            break

        row = tripadvisor_df.iloc[tripadvisor_index]
        name = clean_text(row.get("restaurant_name"), 100)
        normalized = normalize_name(name)
        if not name or normalized in seen_names:
            continue

        if pd.isna(row.get("latitude")) or pd.isna(row.get("longitude")):
            continue

        categories: List[str] = []
        for field in ["top_tags", "cuisines", "meals", "features"]:
            categories.extend(extract_categories(row.get(field)))

        unique_categories: List[str] = []
        seen_categories: set[str] = set()
        for category in categories:
            key = category.lower()
            if key in seen_categories:
                continue
            seen_categories.add(key)
            unique_categories.append(category)

        claimed = clean_text(row.get("claimed")).lower()
        sponsor_tier = 1 if claimed == "claimed" else 0

        yield RestaurantRecord(
            id=next_id,
            name=name,
            latitude=float(row.get("latitude")),
            longitude=float(row.get("longitude")),
            address=clean_text(row.get("address"), 100),
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
        next_id += 1
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
    return list(build_restaurants_stream(places_df, reviews_df, tripadvisor_df, max_restaurants))


def build_restaurant_data(places_df: pd.DataFrame, reviews_df: pd.DataFrame, tripadvisor_df: pd.DataFrame) -> Iterable:
    """Build restaurant records from places, reviews, and tripadvisor data (streaming)."""
    return build_restaurants_stream(
        places_df=places_df,
        reviews_df=reviews_df,
        tripadvisor_df=tripadvisor_df,
        max_restaurants=None,
    )
