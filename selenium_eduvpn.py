import os
import sys
from selenium import webdriver
from selenium.webdriver.common.keys import Keys
from selenium.webdriver.firefox.options import Options


def login_eduvpn(authURL):
    options = Options()
    options.headless = True

    # Use the firefox driver
    driver = webdriver.Firefox(options=options)

    login_oauth(driver, authURL)

    # Cleanup
    driver.close()


# Logs in to the default vpn user portal with selenium
def login_oauth(driver, authURL):
    # Go to the oauth url and verify the title
    driver.get(authURL)
    assert "VPN Portal - Sign In" in driver.title

    # Get credentials
    portal_user = os.getenv("PORTAL_USER")
    if portal_user is None:
        print("Error: No portal username set, set the PORTAL_USER env var")
        sys.exit(1)

    portal_pass = os.getenv("PORTAL_PASS")
    if portal_pass is None:
        print("Error: No portal password set, set the PORTAL_PASS env var")
        sys.exit(1)

    # Fill in username
    elem = driver.find_element("name", "userName")
    elem.clear()
    elem.send_keys(portal_user)

    # Fill in password
    elem = driver.find_element("name", "userPass")
    elem.clear()
    elem.send_keys(portal_pass)

    # Click login button
    driver.find_element(
        "css selector", ".frm > fieldset:nth-child(2) > button:nth-child(2)"
    ).click()

    # Check that we have reached the approve application page
    assert "VPN Portal - Approve Application" in driver.title
    # Click approve
    driver.find_element(
        "css selector", ".frm > fieldset:nth-child(1) > button:nth-child(1)"
    ).click()


if __name__ == "__main__":
    # Invalid arguments
    if len(sys.argv) != 2:
        print("Error: no auth url specified")
        sys.exit(1)

    # Login to the portal
    authURL = sys.argv[1]
    login_eduvpn(authURL)
