import csv
from pathlib import Path
from typing import List, Optional

import pandas as pd

from .service_seed_common import DEFAULT_PASSWORD_HASH, print_progress_end, print_progress_start, print_progress_step, slugify_username


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
                "user_id": len(users) + 1,
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

            written += 1
            writer.writerow(
                {
                    "user_id": written,
                    "username": username,
                    "password": DEFAULT_PASSWORD_HASH,
                    "email": email,
                }
            )

            last_pct = print_progress_step("Writing auth CSV", written, total_rows, last_pct)

    if last_pct < 100:
        print_progress_end("Writing auth CSV")

    return written
