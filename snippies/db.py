import sqlite3
from datetime import datetime
from snippies import app_log


def init_db():
    app_log.debug("Initializing backup schedules database")
    with sqlite3.connect('/config/schedules.sqlite') as db:
        db.execute("""
            CREATE TABLE IF NOT EXISTS schedules (
                id INTEGER PRIMARY KEY,
                username TEXT NOT NULL,
                date TEXT NOT NULL,
                action TEXT NOT NULL
            );
        """)


def get_records():
    app_log.debug("Getting backup records")
    with sqlite3.connect('/config/schedules.sqlite') as database:
        rows = database.execute(
            "SELECT id, username, date, action FROM schedules").fetchall()
        if len(rows) == 0:
            app_log.debug("No records found")
            return None
        else:
            return rows


def add_record(username, date, action):
    app_log.debug("Formatting datetime object to string")
    date = datetime.strftime(date, "%Y-%m-%dT%H:%M:%S.000Z")
    app_log.debug(f"Adding record ({username}, {date}, {action}) into database")
    with sqlite3.connect('/config/schedules.sqlite') as database:
        cur = database.execute("INSERT INTO schedules (username, date, action) VALUES (?, ?, ?)",
                               (username, date, action))
        return cur.lastrowid


def delete_record(row_id):
    app_log.debug(f"Attempting to delete record {row_id}")
    with sqlite3.connect('/config/schedules.sqlite') as database:
        database.execute("DELETE from schedules WHERE id = ?", (row_id,))
