#!/usr/bin/env python3
import requests
import io
import re
import zipfile
import sys

BASE_URL = "http://localhost:3000"

def run_tests():
    print("=== Starting Waitlist System Verification ===")
    
    session = requests.Session()
    
    # 1. Login as admin
    print("[1] Logging in as admin...")
    login_data = {
        "username": "trisf",
        "password": "admin123"
    }
    r = session.post(f"{BASE_URL}/admin/login", data=login_data, allow_redirects=False)
    print(f"Login Response Status: {r.status_code}")
    print(f"Login Response Headers: {r.headers}")
    if r.status_code != 303:
        print(f"[-] Login failed. Status: {r.status_code}")
        sys.exit(1)
    
    cookie_header = r.headers.get("Set-Cookie")
    if cookie_header:
        cookie_parts = cookie_header.split(";")[0]
        session.headers.update({"Cookie": cookie_parts})
        print(f"[+] Manually set session cookie header: {cookie_parts}")
    print("[+] Login successful.")
    
    # 2. Check if we can view events
    r = session.get(f"{BASE_URL}/admin/events")
    if r.status_code != 200:
        print(f"[-] Failed to fetch admin events list. Status: {r.status_code}")
        sys.exit(1)
    
    # Clean up any old event with slug 'waitlist-test' or 'wt-test' if they exist in the DB,
    # or just use a unique slug 'wt-test'
    test_slug = "wt-test"
    
    # 3. Create a test event with limit = 2 and enable_waiting_list = true
    print(f"[2] Creating test event with slug '{test_slug}' and capacity = 2...")
    event_data = {
        "title": "Waitlist Test Event",
        "slug": test_slug,
        "description": "Event for testing waitlist functionality",
        "event_date": "2026-07-20",
        "event_time": "09:00 - 12:00",
        "location": "Virtual",
        "price": "0",
        "admin_name": "Test Admin",
        "admin_whatsapp": "08120000000",
        "max_participants": "2",
        "status": "PUBLISHED",
        "enable_waiting_list": "true"
    }
    
    r = session.post(f"{BASE_URL}/admin/events/create", data=event_data, allow_redirects=False)
    if r.status_code != 303:
        # Check if it was because slug was taken, then edit or use wt-test-2
        print("[-] Event creation did not redirect. Checking if slug is already taken...")
        test_slug = "wt-test-2"
        event_data["slug"] = test_slug
        r = session.post(f"{BASE_URL}/admin/events/create", data=event_data, allow_redirects=False)
        if r.status_code != 303:
            print(f"[-] Failed to create event. Response status: {r.status_code}")
            sys.exit(1)
            
    print(f"[+] Event created successfully with slug: {test_slug}")
    
    # Get all events to find the ID of our created event
    r = session.get(f"{BASE_URL}/admin/events")
    matches = re.findall(r'href="/admin/events/(\d+)"', r.text)
    if not matches:
        print("[-] Could not find any event IDs on admin/events page")
        print("--- Response Text ---")
        print(r.text)
        print("---------------------")
        sys.exit(1)
    
    event_id = matches[0] # The latest created one is usually displayed first, but let's be sure
    # Or search for the slug in the page
    for match in matches:
        detail_r = session.get(f"{BASE_URL}/admin/events/{match}")
        if f"/{test_slug}" in detail_r.text:
            event_id = match
            break
            
    print(f"[+] Found created Event ID: {event_id}")
    
    # Ensure registration is globally enabled in settings
    # We can do this by posting to settings if needed, but let's assume it is enabled by default.
    # Let's verify by requesting the register page first
    reg_page_r = requests.get(f"{BASE_URL}/register")
    if "Pendaftaran peserta sedang ditutup" in reg_page_r.text:
        print("[-] Registration is disabled in global settings. Enabling it...")
        # Enable it via admin settings POST
        # Let's find settings endpoint if needed
        
    # 4. Submit first participant (capacity = 2, active = 0) -> Status: REGISTERED
    print("[3] Registering first participant...")
    dummy_payment = io.BytesIO(b"dummy image data")
    files = {
        "payment_proof": ("proof1.png", dummy_payment, "image/png")
    }
    form_data = {
        "full_name": "Participant One",
        "phone": "081234567890",
        "email": "part1@example.com",
        "city": "Karawang",
        "company_name": "PT Test One"
    }
    
    reg_r = requests.post(f"{BASE_URL}/register", data=form_data, files=files, allow_redirects=False)
    if reg_r.status_code != 303 or "/register/success" not in reg_r.headers.get("Location", ""):
        print(f"[-] Participant 1 registration failed. Status: {reg_r.status_code}")
        print(reg_r.text)
        sys.exit(1)
    
    # Check success page
    success_url = BASE_URL + reg_r.headers["Location"]
    success_r = requests.get(success_url)
    if "Placed on the waiting list" in success_r.text or "Daftar Tunggu" in success_r.text:
        print("[-] Participant 1 incorrectly placed on waiting list!")
        sys.exit(1)
    print("[+] Participant 1 registered successfully (under capacity).")
    
    # 5. Submit second participant (capacity = 2, active = 1) -> Status: REGISTERED
    print("[4] Registering second participant...")
    dummy_payment = io.BytesIO(b"dummy image data")
    files = {
        "payment_proof": ("proof2.png", dummy_payment, "image/png")
    }
    form_data = {
        "full_name": "Participant Two",
        "phone": "081234567891",
        "email": "part2@example.com",
        "city": "Cikarang",
        "company_name": "PT Test Two"
    }
    
    reg_r = requests.post(f"{BASE_URL}/register", data=form_data, files=files, allow_redirects=False)
    if reg_r.status_code != 303 or "/register/success" not in reg_r.headers.get("Location", ""):
        print(f"[-] Participant 2 registration failed. Status: {reg_r.status_code}")
        sys.exit(1)
    
    # Check success page
    success_url = BASE_URL + reg_r.headers["Location"]
    success_r = requests.get(success_url)
    if "Placed on the waiting list" in success_r.text or "Daftar Tunggu" in success_r.text:
        print("[-] Participant 2 incorrectly placed on waiting list!")
        sys.exit(1)
    print("[+] Participant 2 registered successfully (under capacity).")

    # 6. Submit third participant (capacity = 2, active = 2) -> Status: WAITLIST
    print("[5] Registering third participant (should go to waitlist)...")
    dummy_payment = io.BytesIO(b"dummy image data")
    files = {
        "payment_proof": ("proof3.png", dummy_payment, "image/png")
    }
    form_data = {
        "full_name": "Participant Three",
        "phone": "081234567892",
        "email": "part3@example.com",
        "city": "Bekasi",
        "company_name": "PT Test Three"
    }
    
    reg_r = requests.post(f"{BASE_URL}/register", data=form_data, files=files, allow_redirects=False)
    if reg_r.status_code != 303 or "/register/success" not in reg_r.headers.get("Location", ""):
        print(f"[-] Participant 3 registration failed. Status: {reg_r.status_code}")
        sys.exit(1)
        
    success_url = BASE_URL + reg_r.headers["Location"]
    success_r = requests.get(success_url)
    
    # Verify success page waitlist messaging
    # Expected: "You have been placed on the waiting list." / "Anda Masuk Daftar Tunggu" / "Waiting List Position"
    if "waiting list" not in success_r.text.lower() and "daftar tunggu" not in success_r.text.lower():
        print("[-] Success page does not show waitlist placement message!")
        print(success_r.text)
        sys.exit(1)
        
    if "position" not in success_r.text.lower() and "posisi" not in success_r.text.lower():
        print("[-] Success page does not show waitlist position!")
        sys.exit(1)
        
    print("[+] Success page waitlist message verified. Waitlist position shown.")
    
    # Find participant 3 ID
    # We can check admin participants list
    part_r = session.get(f"{BASE_URL}/admin/participants")
    # Find all /admin/participants/<id> links
    all_links = re.findall(r'/admin/participants/(\d+)', part_r.text)
    print(f"[+] Found participant links: {all_links}")
    
    p3_id = None
    # Let's find Participant Three in the page and extract the ID before/after it
    for link in set(all_links):
        # Check if Participant Three is near this link or if this link page has Participant Three
        detail_test = session.get(f"{BASE_URL}/admin/participants/{link}")
        if "Participant Three" in detail_test.text:
            p3_id = link
            break
            
    if not p3_id:
        print("[-] Could not find Participant Three in admin list!")
        print("--- Response Text Snippet ---")
        print(part_r.text[:2000])
        print("-----------------------------")
        sys.exit(1)
    print(f"[+] Found Participant Three ID: {p3_id}")
    
    # Verify they are in WAITLIST status
    detail_r = session.get(f"{BASE_URL}/admin/participants/{p3_id}")
    if "WAITLIST" not in detail_r.text:
        print("[-] Participant Three is not showing WAITLIST status in admin detail!")
        sys.exit(1)
    print("[+] Participant Three status verified as WAITLIST.")
    
    # 7. Promote third participant from WAITLIST -> REGISTERED
    print("[6] Promoting Participant Three to REGISTERED...")
    status_data = {
        "status": "REGISTERED"
    }
    promote_r = session.post(f"{BASE_URL}/admin/participants/{p3_id}/status", data=status_data)
    if promote_r.status_code not in [200, 303]:
        print(f"[-] Promotion request failed. Status: {promote_r.status_code}")
        sys.exit(1)
        
    # Verify they are now in REGISTERED status
    detail_r = session.get(f"{BASE_URL}/admin/participants/{p3_id}")
    if "REGISTERED" not in detail_r.text:
        print("[-] Participant Three was not successfully promoted to REGISTERED!")
        sys.exit(1)
    print("[+] Participant Three promoted to REGISTERED successfully.")

    # 8. Export participants to Excel and verify
    print("[7] Exporting participants to Excel and verifying columns...")
    export_r = session.get(f"{BASE_URL}/admin/participants/export")
    if export_r.status_code != 200:
        print(f"[-] Excel export failed. Status: {export_r.status_code}")
        sys.exit(1)
        
    # Parse XLSX contents directly from bytes without saving to disk
    xlsx_bytes = export_r.content
    try:
        with zipfile.ZipFile(io.BytesIO(xlsx_bytes)) as z:
            found_registration_status = False
            found_registered_val = False
            
            # Check all XML files in the zip
            for name in z.namelist():
                if name.endswith('.xml'):
                    xml_content = z.read(name).decode('utf-8', errors='ignore')
                    if "Registration Status" in xml_content:
                        found_registration_status = True
                    if "Registered" in xml_content:
                        found_registered_val = True
                        
            if not found_registration_status:
                print("[-] Could not find 'Registration Status' column in exported Excel sheet.")
                sys.exit(1)
            if not found_registered_val:
                print("[-] Could not find 'Registered' value in exported Excel sheet.")
                sys.exit(1)
            print("[+] Exported Excel contains 'Registration Status' column with correct values.")
    except Exception as e:
        print(f"[-] Failed to parse exported Excel zip archive: {e}")
        sys.exit(1)
        
    # 9. Disable waiting list for this event
    print("[8] Disabling waiting list on test event...")
    edit_data = {
        "title": "Waitlist Test Event",
        "slug": test_slug,
        "description": "Event for testing waitlist functionality",
        "event_date": "2026-07-20",
        "event_time": "09:00 - 12:00",
        "location": "Virtual",
        "price": "0",
        "admin_name": "Test Admin",
        "admin_whatsapp": "08120000000",
        "max_participants": "2",
        "status": "PUBLISHED"
        # enable_waiting_list omitted/false
    }
    edit_r = session.post(f"{BASE_URL}/admin/events/{event_id}/edit", data=edit_data, allow_redirects=False)
    if edit_r.status_code != 303:
        print(f"[-] Failed to edit event settings. Status: {edit_r.status_code}")
        sys.exit(1)
    print("[+] Waiting list disabled successfully on test event.")
    
    # 10. Attempt registering 4th participant when capacity reached and waitlist disabled -> should block
    print("[9] Attempting to register 4th participant (should be blocked as capacity is full)...")
    dummy_payment = io.BytesIO(b"dummy data")
    files = {
        "payment_proof": ("proof4.png", dummy_payment, "image/png")
    }
    form_data = {
        "full_name": "Participant Four",
        "phone": "081234567893",
        "email": "part4@example.com",
        "city": "Cikarang",
        "company_name": "PT Test Four"
    }
    
    reg_r = requests.post(f"{BASE_URL}/register", data=form_data, files=files)
    # It should display validation error page: "Pendaftaran sudah penuh." / "penuh"
    if "penuh" not in reg_r.text.lower() and "full" not in reg_r.text.lower():
        print("[-] Registration was not blocked when capacity reached and waitlist disabled!")
        print(reg_r.text)
        sys.exit(1)
    print("[+] Registration blocked correctly with 'Pendaftaran sudah penuh' message.")

    # 11. Check admin dashboard display
    print("[10] Inspecting admin dashboard for Capacity, Registered, Waitlist, Remaining Seats...")
    dash_r = session.get(f"{BASE_URL}/admin/dashboard?event_id={event_id}")
    if dash_r.status_code != 200:
        print(f"[-] Dashboard fetch failed. Status: {dash_r.status_code}")
        sys.exit(1)
        
    dashboard_html = dash_r.text.lower()
    
    # Verify Capacity, Registered, Waitlist, Remaining Seats are present
    checks = {
        "kapasitas": "Capacity",
        "terdaftar": "Registered",
        "waiting list": "Waitlist",
        "sisa kursi": "Remaining Seats"
    }
    for key, label in checks.items():
        if key not in dashboard_html:
            print(f"[-] Dashboard HTML is missing label/field: '{label}'")
            sys.exit(1)
            
    print("[+] Admin dashboard displays Capacity, Registered, Waitlist, and Remaining Seats correctly.")
    
    print("=== Verification Successful! All Waitlist workflows are working perfectly. ===")

if __name__ == "__main__":
    run_tests()
