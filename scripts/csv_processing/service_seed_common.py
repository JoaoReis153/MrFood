from __future__ import annotations

import os
import re
from dataclasses import dataclass
from datetime import datetime, timedelta
from pathlib import Path
from typing import Dict, List, Optional

import pandas as pd

SCRIPT_DIR = Path(__file__).resolve().parent.parent
PROJECT_DIR = SCRIPT_DIR.parent
DATA_DIR = PROJECT_DIR / "data"
OUTPUT_DIR = SCRIPT_DIR / "processed_data"

DEFAULT_PASSWORD_HASH = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"
WEEK_BASE_DATE = datetime(2026, 1, 5)  # Monday


def print_progress_start(label: str) -> None:
    print(f"{label}: 0%")


def print_progress_end(label: str) -> None:
    print(f"{label}: 100%")


def print_progress_step(label: str, done: int, total: int, last_pct: int, step: int = 5) -> int:
    if total <= 0:
        if last_pct < 100:
            print(f"{label}: 100%")
            return 100
        return last_pct

    pct = min((done * 100) // total, 100)
    milestone_pct = (pct // step) * step

    if milestone_pct <= last_pct:
        return last_pct

    for value in range(last_pct + step, milestone_pct + 1, step):
        print(f"{label}: {value}%")

    return milestone_pct


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


@dataclass
class UserRecord:
    user_id: int
    username: str
    password: str
    email: str
    source_gplus_user_id: Optional[str]


def clean_text(value: object, max_len: Optional[int] = None) -> str:
    if pd.isna(value):
        return ""
    text = str(value).strip()
    if not text:
        return ""
    if max_len is not None:
        return text[:max_len]
    return text


def clean_id(value: object) -> str:
    text = clean_text(value)
    if not text:
        return ""

    if text.endswith(".0") and text[:-2].isdigit():
        return text[:-2]

    return text


def normalize_name(name: str) -> str:
    return re.sub(r"\s+", " ", clean_text(name).lower())


def parse_time_token(token: object):
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
    hours_text = clean_text(hours_str)
    if not hours_text or "--" not in hours_text:
        return None, None

    left, right = [part.strip() for part in hours_text.split("--", 1)]
    open_t = parse_time_token(left)
    close_t = parse_time_token(right)
    return open_t, close_t


def compose_address(parts: List[object]) -> str:
    filtered = [clean_text(p) for p in parts if clean_text(p)]
    return ", ".join(filtered)[:100]


def extract_categories(raw_value: object) -> List[str]:
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
    categories_map: Dict[str, set[str]] = {}
    total_reviews = len(reviews_df)
    print_progress_start("Indexing review categories")
    last_pct = 0

    for index, (_, row) in enumerate(reviews_df.iterrows(), start=1):
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


def build_restaurants(
    places_df: pd.DataFrame,
    reviews_df: pd.DataFrame,
    tripadvisor_df: pd.DataFrame,
    max_restaurants: Optional[int],
) -> List[RestaurantRecord]:
    restaurants: List[RestaurantRecord] = []
    seen_names: set[str] = set()
    review_categories = build_review_categories_map(reviews_df)
    review_place_ids = {clean_id(v) for v in reviews_df.get("gPlusPlaceId", pd.Series(dtype=str)).tolist() if clean_id(v)}

    next_id = 1

    prioritized_rows = []
    secondary_rows = []
    total_places = len(places_df)
    print_progress_start("Preparing place priority list")
    last_pct = 0
    for index, (_, row) in enumerate(places_df.iterrows(), start=1):
        row_place_id = clean_id(row.get("gPlusPlaceId"))
        if row_place_id and row_place_id in review_place_ids:
            prioritized_rows.append(row)
        else:
            secondary_rows.append(row)

        last_pct = print_progress_step("Preparing place priority list", index, total_places, last_pct)

    if last_pct < 100:
        print_progress_end("Preparing place priority list")

    place_candidates = prioritized_rows + secondary_rows
    place_limit = len(place_candidates) if max_restaurants is None else min(max_restaurants, len(place_candidates))
    print_progress_start("Building restaurants from places")
    last_pct = 0
    for index, row in enumerate(place_candidates, start=1):
        if max_restaurants is not None and len(restaurants) >= max_restaurants:
            break

        name = clean_text(row.get("name"), 100)
        normalized = normalize_name(name)
        if not name or normalized in seen_names:
            continue

        if pd.isna(row.get("latitude")) or pd.isna(row.get("longitude")):
            continue

        gplus_place_id = clean_id(row.get("gPlusPlaceId"))
        categories = sorted(list(review_categories.get(gplus_place_id, set())))

        restaurants.append(
            RestaurantRecord(
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
        )
        seen_names.add(normalized)
        next_id += 1

        last_pct = print_progress_step("Building restaurants from places", len(restaurants), place_limit, last_pct)

    if last_pct < 100:
        print_progress_end("Building restaurants from places")

    tripadvisor_limit = len(tripadvisor_df)
    if max_restaurants is not None:
        tripadvisor_limit = max(0, max_restaurants - len(restaurants))

    print_progress_start("Building restaurants from TripAdvisor")
    last_pct = 0
    added_tripadvisor = 0

    for _, row in tripadvisor_df.iterrows():
        if max_restaurants is not None and len(restaurants) >= max_restaurants:
            break

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

        restaurants.append(
            RestaurantRecord(
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
        )
        seen_names.add(normalized)
        next_id += 1
        added_tripadvisor += 1
        last_pct = print_progress_step(
            "Building restaurants from TripAdvisor",
            added_tripadvisor,
            tripadvisor_limit,
            last_pct,
        )

    if last_pct < 100:
        print_progress_end("Building restaurants from TripAdvisor")

    return restaurants


def slugify_username(name: object, fallback_prefix: str, index: int) -> str:
    base = clean_text(name)
    if not base:
        return f"{fallback_prefix}_{index}"
    slug = re.sub(r"[^a-zA-Z0-9_]+", "_", base).strip("_")
    if not slug:
        return f"{fallback_prefix}_{index}"
    return slug[:50]


def ensure_output_dir(path: Path) -> None:
    os.makedirs(path, exist_ok=True)
