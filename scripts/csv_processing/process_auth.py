import csv
import re
from pathlib import Path
from typing import List, Optional

import pandas as pd

from .service_seed_common import DEFAULT_PASSWORD_HASH, clean_text, print_progress_end, print_progress_start, print_progress_step


def slugify_username(name: object, fallback_prefix: str, index: int) -> str:
    """Convert a name into a valid username slug."""
    base = clean_text(name)
    if not base:
        return f"{fallback_prefix}_{index}"
    slug = re.sub(r"[^a-zA-Z0-9_]+", "_", base).strip("_")
    if not slug:
        return f"{fallback_prefix}_{index}"
    return slug[:50]


def build_user_id(raw_user_id: object, index: int) -> str:
    """Build a stable user_id from source gPlusUserId with deterministic fallback."""
    source_id = clean_text(raw_user_id)
    if source_id:
        return source_id
    return f"missing_gplus_user_{index}"


def build_auth_users(users_df: pd.DataFrame) -> List[dict]:
    """Build auth user records from users dataframe."""
    users = []
    used_usernames: set[str] = set()
    used_emails: set[str] = set()

    for idx, (_, row) in enumerate(users_df.iterrows(), start=1):
        username_base = slugify_username(row.get("userName"), "user", idx)
        username = username_base
        suffix = 1

        while username.lower() in used_usernames:
            candidate = f"{username_base}_{suffix}"
            username = candidate[:50]
            suffix += 1

        email = f"{username.lower()}@mrfood.local"
        while email.lower() in used_emails:
            suffix += 1
            email = f"{username.lower()}{suffix}@mrfood.local"
        email = email[:100]

        users.append(
            {
                "user_id": build_user_id(row.get("gPlusUserId"), idx),
                "username": username,
                "password": DEFAULT_PASSWORD_HASH,
                "email": email,
            }
        )

        used_usernames.add(username.lower())
        used_emails.add(email.lower())

    return users


def stream_auth_csv(users_csv_path: Path, output_file: Path, nrows: Optional[int] = None) -> int:
    """Generate auth CSV directly from source users CSV using constant memory."""
    output_file.parent.mkdir(parents=True, exist_ok=True)

    total_rows = 0
    with users_csv_path.open("r", newline="", encoding="utf-8") as input_fp:
        reader = csv.DictReader(input_fp)
        for total_rows, _ in enumerate(reader, start=1):
            if nrows is not None and total_rows >= nrows:
                break

    print_progress_start("Writing auth CSV")
    last_pct = 0
    written = 0

    with (
        users_csv_path.open("r", newline="", encoding="utf-8") as input_fp,
        output_file.open("w", newline="", encoding="utf-8") as output_fp,
    ):
        reader = csv.DictReader(input_fp)
        writer = csv.DictWriter(output_fp, fieldnames=["user_id", "username", "password", "email"])
        writer.writeheader()

        for idx, row in enumerate(reader, start=1):
            if nrows is not None and idx > nrows:
                break

            username_base = slugify_username(row.get("userName"), "user", idx)
            suffix = f"_{idx}"
            base_max_len = max(1, 50 - len(suffix))
            username = f"{username_base[:base_max_len]}{suffix}"[:50]
            email = f"{username.lower()}@mrfood.local"[:100]

            user_id = build_user_id(row.get("gPlusUserId"), idx)
            written += 1
            writer.writerow(
                {
                    "user_id": user_id,
                    "username": username,
                    "password": DEFAULT_PASSWORD_HASH,
                    "email": email,
                }
            )

            last_pct = print_progress_step("Writing auth CSV", written, total_rows, last_pct)

    if last_pct < 100:
        print_progress_end("Writing auth CSV")

    return written


def collect_source_user_ids(users_csv_path: Path, nrows: Optional[int] = None) -> List[str]:
    """Collect ordered source gPlusUserId values for booking generation."""
    ids: List[str] = []
    with users_csv_path.open("r", newline="", encoding="utf-8") as input_fp:
        reader = csv.DictReader(input_fp)
        for idx, row in enumerate(reader, start=1):
            if nrows is not None and idx > nrows:
                break
            ids.append(build_user_id(row.get("gPlusUserId"), idx))
    return ids
