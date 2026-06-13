#!/usr/bin/env python3
import requests
import io
import re
import sys
import time

BASE_URL = "http://localhost:3000"

def run_tests():
    print("=== Starting Sponsor Event Assignment Verification ===")
    
    session = requests.Session()
    
    # 1. Login as admin
    print("[1] Logging in as admin...")
    login_data = {
        "username": "admin",
        "password": "admin123"
    }
    r = session.post(f"{BASE_URL}/admin/login", data=login_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Login failed. Status: {r.status_code}")
        sys.exit(1)
    
    cookie_header = r.headers.get("Set-Cookie")
    if cookie_header:
        cookie_parts = cookie_header.split(";")[0]
        session.headers.update({"Cookie": cookie_parts})
        print(f"[+] Set session cookie header: {cookie_parts}")
    print("[+] Login successful.")
    
    # 2. Create Event A and Event B
    timestamp = int(time.time())
    slug_a = f"evt-a-{timestamp}"
    slug_b = f"evt-b-{timestamp}"
    
    print(f"[2] Creating Event A with slug '{slug_a}'...")
    event_a_data = {
        "title": "Event A Title",
        "slug": slug_a,
        "description": "Event A description",
        "event_date": "2026-08-20",
        "event_time": "09:00 - 12:00",
        "location": "Jakarta",
        "price": "0",
        "admin_name": "Admin A",
        "admin_whatsapp": "08121111111",
        "max_participants": "100",
        "status": "PUBLISHED"
    }
    r = session.post(f"{BASE_URL}/admin/events/create", data=event_a_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Event A. Status: {r.status_code}")
        sys.exit(1)
        
    print(f"[3] Creating Event B with slug '{slug_b}'...")
    event_b_data = {
        "title": "Event B Title",
        "slug": slug_b,
        "description": "Event B description",
        "event_date": "2026-08-21",
        "event_time": "09:00 - 12:00",
        "location": "Bandung",
        "price": "0",
        "admin_name": "Admin B",
        "admin_whatsapp": "08122222222",
        "max_participants": "50",
        "status": "PUBLISHED"
    }
    r = session.post(f"{BASE_URL}/admin/events/create", data=event_b_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Event B. Status: {r.status_code}")
        sys.exit(1)
        
    # Get all events to find the IDs
    r = session.get(f"{BASE_URL}/admin/events")
    matches = re.findall(r'href="/admin/events/(\d+)"', r.text)
    if len(matches) < 2:
        print("[-] Could not find at least two event IDs on admin/events page")
        sys.exit(1)
        
    event_a_id = None
    event_b_id = None
    
    for match in matches:
        detail_r = session.get(f"{BASE_URL}/admin/events/{match}")
        if slug_a in detail_r.text:
            event_a_id = match
        elif slug_b in detail_r.text:
            event_b_id = match
            
    if not event_a_id or not event_b_id:
        print(f"[-] Could not find Event IDs. Event A ID: {event_a_id}, Event B ID: {event_b_id}")
        sys.exit(1)
        
    print(f"[+] Found Event A ID: {event_a_id}, Event B ID: {event_b_id}")
    
    # 3. Create Sponsor A linked to Event A
    print("[4] Creating Sponsor A linked to Event A...")
    logo_file = io.BytesIO(b"dummy image data")
    files_a = {
        "logo": ("sponsor_a.png", logo_file, "image/png")
    }
    sponsor_a_data = {
        "event_id": event_a_id,
        "name": "Sponsor Alpha",
        "category": "Title Sponsor",
        "website_url": "https://alpha.com",
        "display_order": "1",
        "active": "true"
    }
    r = session.post(f"{BASE_URL}/admin/sponsors/create", data=sponsor_a_data, files=files_a, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Sponsor A. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Sponsor A created successfully.")
    
    # 4. Create Sponsor B linked to Event B
    print("[5] Creating Sponsor B linked to Event B...")
    logo_file_2 = io.BytesIO(b"dummy image data 2")
    files_b = {
        "logo": ("sponsor_b.png", logo_file_2, "image/png")
    }
    sponsor_b_data = {
        "event_id": event_b_id,
        "name": "Sponsor Beta",
        "category": "Media Partner",
        "website_url": "https://beta.com",
        "display_order": "2",
        "active": "true"
    }
    r = session.post(f"{BASE_URL}/admin/sponsors/create", data=sponsor_b_data, files=files_b, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Sponsor B. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Sponsor B created successfully.")
    
    # 5. Verify event_id selection required validation
    print("[6] Verifying validation: event_id is required...")
    invalid_logo = io.BytesIO(b"dummy image data 3")
    files_invalid = {
        "logo": ("sponsor_c.png", invalid_logo, "image/png")
    }
    invalid_sponsor_data = {
        "event_id": "", # empty event_id
        "name": "Sponsor Gamma",
        "category": "Gold Sponsor"
    }
    r = session.post(f"{BASE_URL}/admin/sponsors/create", data=invalid_sponsor_data, files=files_invalid)
    if "Acara wajib dipilih" not in r.text:
        print("[-] Validation failed: Sponsor created without event_id selection")
        sys.exit(1)
    print("[+] Validation verified: Empty event_id blocked.")
    
    # 6. Verify database validation for invalid event_id
    print("[7] Verifying validation: event_id must exist in database...")
    invalid_logo_2 = io.BytesIO(b"dummy image data 4")
    files_invalid_2 = {
        "logo": ("sponsor_c.png", invalid_logo_2, "image/png")
    }
    invalid_sponsor_data_2 = {
        "event_id": "99999", # non-existent event_id
        "name": "Sponsor Gamma",
        "category": "Gold Sponsor"
    }
    r = session.post(f"{BASE_URL}/admin/sponsors/create", data=invalid_sponsor_data_2, files=files_invalid_2)
    if "Acara tidak ditemukan" not in r.text:
        print("[-] Validation failed: Sponsor created with non-existent event_id")
        sys.exit(1)
    print("[+] Validation verified: Non-existent event_id blocked.")
    
    # 7. Verify Sponsor list filtering by Event A
    print(f"[8] Fetching sponsors list filtered by Event A (ID: {event_a_id})...")
    r_list_a = session.get(f"{BASE_URL}/admin/sponsors?event_id={event_a_id}")
    if r_list_a.status_code != 200:
        print(f"[-] Failed to fetch sponsors list. Status: {r_list_a.status_code}")
        sys.exit(1)
    if "Sponsor Alpha" not in r_list_a.text:
        print("[-] Sponsor Alpha not visible in Event A sponsor list")
        sys.exit(1)
    if "Sponsor Beta" in r_list_a.text:
        print("[-] Sponsor Beta is leaking into Event A sponsor list")
        sys.exit(1)
    print("[+] Event A sponsors list filtered correctly.")
    
    # 8. Verify Sponsor list filtering by Event B
    print(f"[9] Fetching sponsors list filtered by Event B (ID: {event_b_id})...")
    r_list_b = session.get(f"{BASE_URL}/admin/sponsors?event_id={event_b_id}")
    if r_list_b.status_code != 200:
        print(f"[-] Failed to fetch sponsors list. Status: {r_list_b.status_code}")
        sys.exit(1)
    if "Sponsor Beta" not in r_list_b.text:
        print("[-] Sponsor Beta not visible in Event B sponsor list")
        sys.exit(1)
    if "Sponsor Alpha" in r_list_b.text:
        print("[-] Sponsor Alpha is leaking into Event B sponsor list")
        sys.exit(1)
    print("[+] Event B sponsors list filtered correctly.")
    
    print("=== Verification Successful! Sponsor Event Assignment is working perfectly. ===")

if __name__ == "__main__":
    run_tests()
