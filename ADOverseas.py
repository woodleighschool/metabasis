from flask import Flask, request, jsonify
from datetime import datetime, timezone
from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.jobstores.sqlalchemy import SQLAlchemyJobStore
from apscheduler.executors.pool import ThreadPoolExecutor, ProcessPoolExecutor
from snippies import ad, app_log, db, scheduler_config
import os

app = Flask(__name__)
scheduler = BackgroundScheduler(jobstores=scheduler_config.jobstores,
                                executors=scheduler_config.executors,
                                job_defaults=scheduler_config.job_defaults)
scheduler.start()

app_log.configure_logging()

db.init_db()


def reschedule_jobs():
    app_log.debug("Checking for previously uncompleted jobs")
    app_log.debug("Getting all records from schedules.sqlite")
    rows = db.get_records()

    if rows != None:
        for row in rows:
            rowID, username, date, action = row
            if action == "leaving":
                if datetime.strptime(date+"+0000", "%Y-%m-%dT%H:%M:%S.000Z%z") > datetime.now(timezone.utc):
                    app_log.debug(f"Scheduling leaving job for {username} at {date}")
                    scheduler.add_job(ad.edit_ad_user, id=f"{username}_away", trigger='date', run_date=date, args=[
                        username, 'away', rowID], replace_existing=True)
                else:
                    app_log.debug(f"Job for {username} is in the past, running now instead")
                    scheduler.add_job(ad.edit_ad_user, id=f"{username}_away", args=[username, 'away', rowID], replace_existing=True)
            elif action == "returning":
                if datetime.strptime(date+"+0000", "%Y-%m-%dT%H:%M:%S.000Z%z") > datetime.now(timezone.utc):
                    app_log.debug(f"Scheduling leaving job for {username} at {date}")
                    scheduler.add_job(ad.edit_ad_user, id=f"{username}_home", trigger='date', run_date=date, args=[
                        username, 'home', rowID], replace_existing=True)
    else:
        app_log.debug("No previously uncompleted jobs")


reschedule_jobs()


@app.route('/schedule', methods=['POST'])
def schedule_user():
    api_token = os.getenv('API_TOKEN') # should error out if no token is found
    
    api_key = request.headers.get('Authorization')

    # continue if api_key is correct
    app_log.debug("Checking if authorized")
    if api_key == f'Bearer {api_token}':
        app_log.debug("Getting request as JSON")
        data = request.get_json()

        app_log.debug("Parsing JSON fields")
        email = data.get('username')
        app_log.debug(f"Formatting email {email} into samAccountName")
        username = ad.format_username(email)
        start_date_str = data.get('start_date')
        end_date_str = data.get('end_date')

        # ensure all required fields are sent
        app_log.debug("Checking if all fields have been met")
        if not (username and start_date_str and end_date_str):
            app_log.error(f"Missing a parameter: {request.data}")
            return jsonify({'status': 'request failed', 'reason': 'Missing Parameter'}), 400

        app_log.debug("Attempting to parse dates to datetime object")
        try:
            start_date = datetime.strptime(start_date_str + "+0000", "%Y-%m-%dT%H:%M:%S.000Z%z")
            end_date = datetime.strptime(end_date_str + "+0000", "%Y-%m-%dT%H:%M:%S.000Z%z")
        except ValueError:  # handle incorrect datetime format
            app_log.error(f"Unable to parse dates {start_date_str} and {end_date_str}")
            return jsonify({'status': 'request failed', 'reason': 'invalid date/time format'}), 400

        # make sure that end_date is after start_date
        app_log.debug("Checking if end_date is before start_date")
        if end_date <= start_date:
            app_log.error(f"end_date {end_date} is before start_date {start_date}")
            return jsonify({'status': 'request failed', 'reason': 'end_date cannot be before or same as start_date!'}), 400

        # add to schedule
        app_log.debug("Adding job to scheduler")
        schedule(username, start_date, end_date)
        return jsonify({'status': 'request succesful'})
    app_log.error("API Token does not match")
    return jsonify({'status': 'request failed', 'reason': 'unauthorized'}), 401


def schedule(username, start_date, end_date):
    if start_date <= datetime.now(timezone.utc):
        app_log.debug(f"Start date is in the past, adding {username} to group now instead")
        scheduler.add_job(ad.edit_ad_user, id=f"{username}_away_{start_date}", args=[username, 'away', None], replace_existing=True)
    else:
        app_log.debug(f"Adding data for job {username}_away to schedules database in case of system shutdown")
        row_id = db.add_record(username, start_date, "leaving")
        app_log.debug(f"Scheduling job {username}_away")
        scheduler.add_job(ad.edit_ad_user, id=f"{username}_away_{start_date}", trigger='date', run_date=start_date, timezone=timezone.utc, args=[
            username, 'away', row_id], replace_existing=True)

    app_log.debug(f"Adding data for job {username}_home to schedules database in case of system shutdown")
    row_id = db.add_record(username, end_date, "returning")
    app_log.debug(f"Scheduling job {username}_home")
    scheduler.add_job(ad.edit_ad_user, id=f"{username}_home_{end_date}", trigger='date', run_date=end_date, timezone=timezone.utc, args=[
        username, 'home', row_id], replace_existing=True)

@app.route('/health', methods=['GET'])
def healthCheck():
    return jsonify({'status': 'Success!'}), 200

if __name__ == "__main__":
    app.run()
