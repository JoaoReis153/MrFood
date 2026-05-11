import pandas as pd
import os

# Paths
BASE_DIR = os.path.dirname(os.path.realpath(__file__))
DATA_DIR = os.path.join(BASE_DIR, '../../', 'data')
OUTPUT_DIR = os.path.join(DATA_DIR, 'processed')

INPUT_FILE = os.path.join(DATA_DIR, 'places.csv')
OUTPUT_FILE = os.path.join(OUTPUT_DIR, 'places_clean.csv')


def build_address(row):
    parts = [
        row.get('address_line1'),
        row.get('address_line2'),
        row.get('address_line3')
    ]
    return ", ".join([str(p) for p in parts if pd.notna(p) and p != ""])


def parse_hours(hours_str):
    """
    Converts '6:30 am--4:15 pm' → ('06:30', '16:15')
    """
    if pd.isna(hours_str):
        return None, None

    try:
        open_time, close_time = hours_str.split("--")

        open_time = pd.to_datetime(open_time.strip()).strftime("%H:%M")
        close_time = pd.to_datetime(close_time.strip()).strftime("%H:%M")

        return open_time, close_time
    except:
        return None, None


def process(df):

    print(f"Initial rows: {len(df)}")

    # Drop rows with missing critical data
    df = df.dropna(subset=['name', 'latitude', 'longitude'])

    # Remove duplicates by name
    df = df.drop_duplicates(subset=['name'])

    # Build address
    df['address'] = df.apply(build_address, axis=1)

    # Default fields (for your DB schema)
    df['media_url'] = None
    df['max_slots'] = 50
    df['owner_id'] = 1
    df['owner_name'] = "admin"
    df['sponsor_tier'] = 0

    # Extract working hours into structured columns
    DAYS = [
        "monday", "tuesday", "wednesday",
        "thursday", "friday", "saturday", "sunday"
    ]

    for day in DAYS:
        col = f"hours_{day}"
        df[f"{day}_open"], df[f"{day}_close"] = zip(*df[col].map(parse_hours))

    # Select only relevant columns for restaurants table
    restaurants_df = df[[
        'name', 'latitude', 'longitude', 'address',
        'media_url', 'max_slots', 'owner_id',
        'owner_name', 'sponsor_tier'
    ]]

    # Create output folder
    os.makedirs(OUTPUT_DIR, exist_ok=True)

    print("Saving cleaned restaurants...")
    restaurants_df.to_csv(OUTPUT_FILE, index=False)

    print(f"Processed rows: {len(restaurants_df)}")
    print(f"Saved to: {OUTPUT_FILE}")


if __name__ == "__main__":
    process()