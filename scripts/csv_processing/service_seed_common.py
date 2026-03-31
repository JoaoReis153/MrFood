from __future__ import annotations

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
