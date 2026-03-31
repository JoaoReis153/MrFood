import csv
from datetime import datetime
from pathlib import Path
from typing import Dict, Iterator, Optional


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
    gplus_user_id_to_user_id: Dict[str, int],
    gplus_place_id_to_restaurant_id: Dict[str, int],
    nrows: Optional[int] = None,
) -> Iterator[dict]:
    """Yield reviews matching review table schema using gPlus ID mappings to DB-safe INT IDs."""
    if not gplus_place_id_to_restaurant_id or not gplus_user_id_to_user_id:
        return

    seen_pairs = set()
    available_restaurant_ids = list(gplus_place_id_to_restaurant_id.values())
    available_user_ids = list(gplus_user_id_to_user_id.values())
    
    if not available_restaurant_ids:
        return

    with reviews_csv_path.open("r", newline="", encoding="utf-8") as fp:
        reader = csv.DictReader(fp)

        for source_idx, row in enumerate(reader, start=1):
            if nrows is not None and source_idx > nrows:
                break

            gplus_place_id = str(row.get("gPlusPlaceId") or "").strip()
            gplus_user_id = str(row.get("gPlusUserId") or "").strip()

            user_id = gplus_user_id_to_user_id.get(gplus_user_id) if gplus_user_id else None

            # Try to find matching restaurant
            restaurant_id = gplus_place_id_to_restaurant_id.get(gplus_place_id) if gplus_place_id else None

            # Fallback: distribute unmatched reviews across available restaurants
            if not restaurant_id:
                if not available_restaurant_ids:
                    continue
                restaurant_id = available_restaurant_ids[(source_idx - 1) % len(available_restaurant_ids)]

            if not user_id:
                if not available_user_ids:
                    continue
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
