import csv
from datetime import datetime
from pathlib import Path
from typing import Dict, Iterator, List, Optional


def _parse_rating(raw_value: object) -> int:
    """Parse source rating into a SQL-safe 1..5 integer."""
    try:
        value = int(float(str(raw_value).strip()))
    except (TypeError, ValueError):
        return 3

    if value < 1:
        return 1
    if value > 5:
        return 5
    return value


def _parse_created_at(row: dict) -> str:
    """Build created_at timestamp from unixReviewTime."""
    unix_value = row.get("unixReviewTime")
    if not unix_value:
        return datetime.now().strftime("%Y-%m-%d %H:%M:%S")
    
    try:
        # Parse as float to handle values like "1372686659.0"
        timestamp = float(str(unix_value).strip())
        dt = datetime.fromtimestamp(timestamp)
        return dt.strftime("%Y-%m-%d %H:%M:%S")
    except (TypeError, ValueError, OSError, OverflowError):
        return datetime.now().strftime("%Y-%m-%d %H:%M:%S")


def generate_reviews_stream(
    reviews_csv_path: Path,
    user_id_by_source: Dict[str, str],
    restaurant_id_by_source: Dict[str, str],
    available_user_ids: List[str],
    available_restaurant_ids: List[str],
    nrows: Optional[int] = None,
) -> Iterator[dict]:
    """Yield reviews matching review table schema using generated IDs."""
    if not available_restaurant_ids or not available_user_ids:
        return

    seen_pairs = set()

    with reviews_csv_path.open("r", newline="", encoding="utf-8") as fp:
        reader = csv.DictReader(fp)

        for source_idx, row in enumerate(reader, start=1):
            if nrows is not None and source_idx > nrows:
                break

            source_place_id = str(row.get("gPlusPlaceId") or "").strip()
            source_user_id = str(row.get("gPlusUserId") or "").strip()

            restaurant_id = restaurant_id_by_source.get(source_place_id) if source_place_id else None
            user_id = user_id_by_source.get(source_user_id) if source_user_id else None

            if not restaurant_id:
                restaurant_id = available_restaurant_ids[(source_idx - 1) % len(available_restaurant_ids)]
            if not user_id:
                user_id = available_user_ids[(source_idx - 1) % len(available_user_ids)]

            pair = (restaurant_id, user_id)
            if pair in seen_pairs:
                continue

            seen_pairs.add(pair)
            comment = str(row.get("reviewText") or "").strip()
            if not comment:
                comment = "No comment"

            yield {
                "review_id": len(seen_pairs),
                "restaurant_id": restaurant_id,
                "user_id": user_id,
                "comment": comment[:100],
                "rating": _parse_rating(row.get("rating")),
                "created_at": _parse_created_at(row),
            }
