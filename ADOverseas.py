from flask import Flask, request, jsonify
from datetime import datetime, timezone
from apscheduler.schedulers.background import BackgroundScheduler
from apscheduler.jobstores.sqlalchemy import SQLAlchemyJobStore
from apscheduler.executors.pool import ThreadPoolExecutor, ProcessPoolExecutor
from ldap3 import Server, Connection, ALL
import os
import logging
import sqlite3

app = Flask(__name__)
scheduler = BackgroundScheduler(
	jobstores = {
    'default': SQLAlchemyJobStore(url='sqlite:////app/jobs.sqlite')
	},
    executors = {
    'default': ThreadPoolExecutor(20),
    'processpool': ProcessPoolExecutor(5)
	},
    job_defaults = {
    'coalesce': False,
    'max_instances': 3
	})
scheduler.start()

# Configure logging
log_level = os.getenv('LOG_LEVEL', 'INFO').upper()
numeric_level = getattr(logging, log_level, None)
if not isinstance(numeric_level, int):
	raise ValueError(f'Invalid log level: {log_level}')
logging.basicConfig(level=numeric_level, format='%(asctime)s - %(levelname)s - %(message)s')

class Database():
	def init_db():
		logging.debug("Initializing backup schedules database")
		with sqlite3.connect('/config/schedules.sqlite') as database:
			database.execute("""
				CREATE TABLE IF NOT EXISTS schedules (
					id INTEGER PRIMARY KEY,
					username TEXT NOT NULL,
					date TEXT NOT NULL,
					action TEXT NOT NULL
				);
			""")


	def get_records():
		logging.debug("Getting backup records")
		with sqlite3.connect('/config/schedules.sqlite') as database:
			rows = database.execute(
				"SELECT id, username, date, action FROM schedules").fetchall()
			if len(rows) == 0:
				logging.debug("No records found")
				return None
			else:
				return rows


	def add_record(username, date, action):
		logging.debug("Formatting datetime object to string")
		date = datetime.strftime(date, "%Y-%m-%dT%H:%M:%S.000Z")
		logging.debug(f"Adding record ({username}, {date}, {action}) into database")
		with sqlite3.connect('/config/schedules.sqlite') as database:
			cur = database.execute("INSERT INTO schedules (username, date, action) VALUES (?, ?, ?)",
								(username, date, action))
			return cur.lastrowid


	def delete_record(row_id):
		logging.debug(f"Attempting to delete record {row_id}")
		with sqlite3.connect('/config/schedules.sqlite') as database:
			database.execute("DELETE from schedules WHERE id = ?", (row_id,))

class ActiveDirectoryIntegration():
	def __init__(self):
		# AD Configuration
		self.AD_SERVER = 'WDC01.woodleighschool.net'
		self.AD_USERNAME = os.getenv('AD_USERNAME', '')
		self.AD_PASSWORD = os.getenv('AD_PASSWORD', '')
		self.DOMAIN_BASE = 'DC=woodleighschool,DC=net'


	def connect_to_ad(self):
		logging.debug("Connecting to AD...")
		server = Server(self.AD_SERVER, get_info=ALL, use_ssl=True)
		conn = Connection(server, user=self.AD_USERNAME,
						password=self.AD_PASSWORD, auto_bind=True)
		logging.debug("Connected.")
		return conn


	def edit_ad_user(self, user_identifier, action, db_row):
		conn = self.connect_to_ad()

		logging.debug(f"Looking for user {user_identifier}")
		# Search for the user using their email
		conn.search(search_base=self.DOMAIN_BASE,
					search_filter=f'(sAMAccountName={user_identifier})',
					attributes=['cn', 'department'])

		if not conn.entries:
			logging.error(f"User {user_identifier} not found")
			print(f"Username '{user_identifier}' not found.")
			return

		user_dn = conn.entries[0].entry_dn
		user_department = conn.entries[0].department

		if action == "away":
			logging.debug(f"Adding {user_identifier} to Allow Access When Overseas")
			# add user to overseas access group
			conn.extend.microsoft.add_members_to_groups(
				user_dn, f'CN=Azure - Allow access when overseas (Travelling Users),OU=Office 365 and Azure AD,{self.DOMAIN_BASE}')

			logging.debug(f"Removing {user_identifier} from Block Access If Not In Australia")
			# remove user from group that disables overseas access
			conn.extend.microsoft.remove_members_from_groups(
				user_dn, f'CN=Azure - Block Access to O365 if not in Australia,OU=Office 365 and Azure AD,{self.DOMAIN_BASE}')

			logging.debug(f"Adding {user_identifier} to Enable Office 365 MFA")
			# add user to 2fa group
			conn.extend.microsoft.add_members_to_groups(
				user_dn, f'CN=Enable Office 365 MFA,OU=Office 365 and Azure AD,{self.DOMAIN_BASE}')
		elif action == "home":
			logging.debug(f"Adding {user_identifier} to Block Access If Not In Australia")
			# add user to group that disables overseas access
			conn.extend.microsoft.add_members_to_groups(
				user_dn, f'CN=Azure - Block Access to O365 if not in Australia,OU=Office 365 and Azure AD,{self.DOMAIN_BASE}')

			logging.debug(f"Removing {user_identifier} from Allow Access When Overseas")
			# remove user from overseas access group
			conn.extend.microsoft.remove_members_from_groups(
				user_dn, f'CN=Azure - Allow access when overseas (Travelling Users),OU=Office 365 and Azure AD,{self.DOMAIN_BASE}')

			logging.debug(f"Checking if {user_identifier} is a staff member based on department: {user_department}")
			if user_department != "Staff":
				# remove students from 2fa group
				logging.debug(f"{user_identifier} is not staff so removing from Enable Office 365 MFA")
				conn.extend.microsoft.remove_members_from_groups(
					user_dn, f'CN=Enable Office 365 MFA,OU=Office 365 and Azure AD,{self.DOMAIN_BASE}')

		conn.unbind()
		if db_row != None:
			logging.debug(f"Deleting backup record in schedules.sqlite")
			Database.delete_record(db_row)


	def format_username(email):
		username = email.split('@')[0]
		return username


def reschedule_jobs():
	logging.debug("Checking for previously uncompleted jobs")
	logging.debug("Getting all records from schedules.sqlite")
	rows = Database.get_records()

	if rows != None:
		for row in rows:
			rowID, username, date, action = row
			if action == "leaving":
				if datetime.strptime(date+"+0000", "%Y-%m-%dT%H:%M:%S.000Z%z") > datetime.now(timezone.utc):
					logging.debug(f"Scheduling leaving job for {username} at {date}")
					scheduler.add_job(adIntegration.edit_ad_user, id=f"{username}_away", trigger='date', run_date=date, args=[
						username, 'away', rowID], replace_existing=True)
				else:
					logging.debug(f"Job for {username} is in the past, running now instead")
					scheduler.add_job(adIntegration.edit_ad_user, id=f"{username}_away", args=[username, 'away', rowID], replace_existing=True)
			elif action == "returning":
				if datetime.strptime(date+"+0000", "%Y-%m-%dT%H:%M:%S.000Z%z") > datetime.now(timezone.utc):
					logging.debug(f"Scheduling leaving job for {username} at {date}")
					scheduler.add_job(adIntegration.edit_ad_user, id=f"{username}_home", trigger='date', run_date=date, args=[
						username, 'home', rowID], replace_existing=True)
	else:
		logging.debug("No previously uncompleted jobs")

def schedule(username, start_date, end_date):
	if start_date <= datetime.now(timezone.utc):
		logging.debug(f"Start date is in the past, adding {username} to group now instead")
		scheduler.add_job(adIntegration.edit_ad_user, id=f"{username}_away_{start_date}", args=[username, 'away', None], replace_existing=True)
	else:
		logging.debug(f"Adding data for job {username}_away to schedules database in case of system shutdown")
		row_id = Database.add_record(username, start_date, "leaving")
		logging.debug(f"Scheduling job {username}_away")
		scheduler.add_job(adIntegration.edit_ad_user, id=f"{username}_away_{start_date}", trigger='date', run_date=start_date, timezone=timezone.utc, args=[
			username, 'away', row_id], replace_existing=True)

	logging.debug(f"Adding data for job {username}_home to schedules database in case of system shutdown")
	row_id = Database.add_record(username, end_date, "returning")
	logging.debug(f"Scheduling job {username}_home")
	scheduler.add_job(adIntegration.edit_ad_user, id=f"{username}_home_{end_date}", trigger='date', run_date=end_date, timezone=timezone.utc, args=[
		username, 'home', row_id], replace_existing=True)



@app.route('/schedule', methods=['POST'])
def schedule_user():
	api_token = os.getenv('API_TOKEN') # should error out if no token is found
	
	api_key = request.headers.get('Authorization')

	# continue if api_key is correct
	logging.debug("Checking if authorized")
	if api_key == f'Bearer {api_token}':
		logging.debug("Getting request as JSON")
		data = request.get_json()

		logging.debug("Parsing JSON fields")
		email = data.get('username')
		logging.debug(f"Formatting email {email} into samAccountName")
		username = ActiveDirectoryIntegration.format_username(email)
		start_date_str = data.get('start_date')
		end_date_str = data.get('end_date')

		# ensure all required fields are sent
		logging.debug("Checking if all fields have been met")
		if not (username and start_date_str and end_date_str):
			logging.error(f"Missing a parameter: {request.data}")
			return jsonify({'status': 'request failed', 'reason': 'Missing Parameter'}), 400

		logging.debug("Attempting to parse dates to datetime object")
		try:
			start_date = datetime.strptime(start_date_str + "+0000", "%Y-%m-%dT%H:%M:%S.000Z%z")
			end_date = datetime.strptime(end_date_str + "+0000", "%Y-%m-%dT%H:%M:%S.000Z%z")
		except ValueError:  # handle incorrect datetime format
			logging.error(f"Unable to parse dates {start_date_str} and {end_date_str}")
			return jsonify({'status': 'request failed', 'reason': 'invalid date/time format'}), 400

		# make sure that end_date is after start_date
		logging.debug("Checking if end_date is before start_date")
		if end_date <= start_date:
			logging.error(f"end_date {end_date} is before start_date {start_date}")
			return jsonify({'status': 'request failed', 'reason': 'end_date cannot be before or same as start_date!'}), 400

		# add to schedule
		logging.debug("Adding job to scheduler")
		schedule(username, start_date, end_date)
		return jsonify({'status': 'request succesful'})
	logging.error("API Token does not match")
	return jsonify({'status': 'request failed', 'reason': 'unauthorized'}), 401



@app.route('/health', methods=['GET'])
def healthCheck():
	return jsonify({'status': 'Success!'}), 200

def main():
	Database.init_db()
	global adIntegration
	adIntegration = ActiveDirectoryIntegration()
	reschedule_jobs()
	app.run(host="0.0.0.0", port=80)

if __name__ == "__main__":
	main()
