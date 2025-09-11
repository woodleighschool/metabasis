from ldap3 import Server, Connection, ALL
from snippies import db, app_log
import os

# AD Configuration
AD_SERVER = 'WDC01.woodleighschool.net'
AD_USERNAME = os.getenv('AD_USERNAME', '')
AD_PASSWORD = os.getenv('AD_PASSWORD', '')
DOMAIN_BASE = 'DC=woodleighschool,DC=net'


def connect_to_ad():
    app_log.debug("Connecting to AD...")
    server = Server(AD_SERVER, get_info=ALL, use_ssl=True)
    conn = Connection(server, user=AD_USERNAME,
                      password=AD_PASSWORD, auto_bind=True)
    app_log.debug("Connected.")
    return conn


def edit_ad_user(user_identifier, action, db_row):
    conn = connect_to_ad()

    app_log.debug(f"Looking for user {user_identifier}")
    # Search for the user using their email
    conn.search(search_base=DOMAIN_BASE,
                search_filter=f'(sAMAccountName={user_identifier})',
                attributes=['cn', 'department'])

    if not conn.entries:
        app_log.error(f"User {user_identifier} not found")
        print(f"Username '{user_identifier}' not found.")
        return

    user_dn = conn.entries[0].entry_dn
    user_department = conn.entries[0].department

    if action == "away":
        app_log.debug(f"Adding {user_identifier} to Allow Access When Overseas")
        # add user to overseas access group
        conn.extend.microsoft.add_members_to_groups(
            user_dn, f'CN=Azure - Allow access when overseas (Travelling Users),OU=Office 365 and Azure AD,{DOMAIN_BASE}')

        app_log.debug(f"Removing {user_identifier} from Block Access If Not In Australia")
        # remove user from group that disables overseas access
        conn.extend.microsoft.remove_members_from_groups(
            user_dn, f'CN=Azure - Block Access to O365 if not in Australia,OU=Office 365 and Azure AD,{DOMAIN_BASE}')

        app_log.debug(f"Adding {user_identifier} to Enable Office 365 MFA")
        # add user to 2fa group
        conn.extend.microsoft.add_members_to_groups(
            user_dn, f'CN=Enable Office 365 MFA,OU=Office 365 and Azure AD,{DOMAIN_BASE}')
    elif action == "home":
        app_log.debug(f"Adding {user_identifier} to Block Access If Not In Australia")
        # add user to group that disables overseas access
        conn.extend.microsoft.add_members_to_groups(
            user_dn, f'CN=Azure - Block Access to O365 if not in Australia,OU=Office 365 and Azure AD,{DOMAIN_BASE}')

        app_log.debug(f"Removing {user_identifier} from Allow Access When Overseas")
        # remove user from overseas access group
        conn.extend.microsoft.remove_members_from_groups(
            user_dn, f'CN=Azure - Allow access when overseas (Travelling Users),OU=Office 365 and Azure AD,{DOMAIN_BASE}')

        app_log.debug(f"Checking if {user_identifier} is a staff member based on department: {user_department}")
        if user_department != "Staff":
            # remove students from 2fa group
            app_log.debug(f"{user_identifier} is not staff so removing from Enable Office 365 MFA")
            conn.extend.microsoft.remove_members_from_groups(
                user_dn, f'CN=Enable Office 365 MFA,OU=Office 365 and Azure AD,{DOMAIN_BASE}')

    conn.unbind()
    if db_row != None:
        app_log.debug(f"Deleting backup record in schedules.sqlite")
        db.delete_record(db_row)


def format_username(email):
    username = email.split('@')[0]
    return username
