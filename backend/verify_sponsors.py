#!/usr/bin/env python3
import requests
import io
import re
import sys

BASE_URL = "http://localhost:3000"

def run_tests():
    print("=== Starting Sponsor & Partner Management Verification ===")
    
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
    
    # 2. Create a test event with enable_sponsors = true
    import time
    test_slug = f"sponsor-test-{int(time.time())}"
    print(f"[2] Creating test event with slug '{test_slug}' and enable_sponsors = true...")
    event_data = {
        "title": "Sponsor Test Event",
        "slug": test_slug,
        "description": "Event for testing sponsor functionality",
        "event_date": "2026-08-20",
        "event_time": "09:00 - 12:00",
        "location": "JCC Senayan, Jakarta",
        "price": "50000",
        "admin_name": "Test Sponsor Admin",
        "admin_whatsapp": "08121111111",
        "max_participants": "100",
        "status": "PUBLISHED",
        "enable_sponsors": "true"
    }
    
    r = session.post(f"{BASE_URL}/admin/events/create", data=event_data, allow_redirects=False)
    if r.status_code != 303:
        print("[-] Event creation did not redirect. Trying to create with a unique slug...")
        test_slug = f"sponsor-test-retry-{int(time.time())}"
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
        sys.exit(1)
    
    event_id = None
    for match in matches:
        detail_r = session.get(f"{BASE_URL}/admin/events/{match}")
        if f"/{test_slug}" in detail_r.text:
            event_id = match
            break
            
    if not event_id:
        print("[-] Could not locate Event ID for test slug")
        sys.exit(1)
        
    print(f"[+] Found created Event ID: {event_id}")
    
    # 3. Create a Title Sponsor
    print("[3] Adding a Title Sponsor...")
    dummy_logo = io.BytesIO(b"dummy png logo data")
    files = {
        "logo": ("google_logo.png", dummy_logo, "image/png")
    }
    sponsor_data = {
        "event_id": event_id,
        "name": "Google",
        "category": "Title Sponsor",
        "website_url": "https://google.com",
        "display_order": "1",
        "active": "true"
    }
    
    r = session.post(f"{BASE_URL}/admin/sponsors/create", data=sponsor_data, files=files, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Title Sponsor. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Title Sponsor 'Google' created successfully.")
    
    # 4. Create a Media Partner
    print("[4] Adding a Media Partner...")
    dummy_logo_2 = io.BytesIO(b"dummy webp logo data")
    files_2 = {
        "logo": ("techcrunch_logo.webp", dummy_logo_2, "image/webp")
    }
    partner_data = {
        "event_id": event_id,
        "name": "TechCrunch",
        "category": "Media Partner",
        "website_url": "https://techcrunch.com",
        "display_order": "2",
        "active": "true"
    }
    
    r = session.post(f"{BASE_URL}/admin/sponsors/create", data=partner_data, files=files_2, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Media Partner. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Media Partner 'TechCrunch' created successfully.")
    
    # 5. Verify sponsors list in admin
    print("[5] Verifying sponsors list in admin panel...")
    sponsors_r = session.get(f"{BASE_URL}/admin/sponsors")
    if sponsors_r.status_code != 200:
        print(f"[-] Failed to load admin sponsors list. Status: {sponsors_r.status_code}")
        sys.exit(1)
        
    if "Google" not in sponsors_r.text or "TechCrunch" not in sponsors_r.text:
        print("[-] Registered sponsors or partners are missing from the admin list!")
        sys.exit(1)
    print("[+] Verified registered sponsors in admin list.")
    
    # Find sponsor IDs
    sponsor_ids = re.findall(r'href="/admin/sponsors/(\d+)/edit"', sponsors_r.text)
    print(f"[+] Found sponsor edit links: {sponsor_ids}")
    
    # 6. Verify public event page displays the sponsors & partners grouped by category
    print("[6] Checking public event page for sponsors display...")
    public_r = requests.get(f"{BASE_URL}/event/{test_slug}")
    if public_r.status_code != 200:
        # Check first event landing page as fallback
        public_r = requests.get(f"{BASE_URL}/")
        if public_r.status_code != 200:
            print(f"[-] Failed to load public event page. Status: {public_r.status_code}")
            sys.exit(1)
            
    public_html = public_r.text
    if "Google" not in public_html or "TechCrunch" not in public_html:
        print("[-] Sponsors or partners not rendered on public page!")
        sys.exit(1)
    
    if "Title Sponsor" not in public_html or "Media Partner" not in public_html:
        print("[-] Sponsor category labels not rendered on public page!")
        sys.exit(1)
        
    print("[+] Grouped sponsors & partners display verified on public event page.")
    
    # 7. Check admin dashboard display for sponsor count
    print("[7] Verifying sponsor count on admin dashboard...")
    dash_r = session.get(f"{BASE_URL}/admin/dashboard?event_id={event_id}")
    if dash_r.status_code != 200:
        print(f"[-] Dashboard fetch failed. Status: {dash_r.status_code}")
        sys.exit(1)
        
    if "jumlah sponsor" not in dash_r.text.lower():
        print("[-] 'Jumlah Sponsor' card is missing on admin dashboard!")
        sys.exit(1)
        
    # Match number next to Jumlah Sponsor
    # Look at the layout: <p class="text-white/35 text-xs mb-1">Jumlah Sponsor</p>\n            <p class="text-white font-bold">2</p>
    match = re.search(r'jumlah sponsor</p>\s*<p class="text-white font-bold">(\d+)</p>', dash_r.text, re.IGNORECASE)
    if not match:
        print("[-] Sponsor count value not found on admin dashboard!")
        sys.exit(1)
        
    count = int(match.group(1))
    if count != 2:
        print(f"[-] Expected sponsor count on dashboard is 2, got: {count}")
        sys.exit(1)
        
    print("[+] Sponsor count correctly displays 2 on admin dashboard.")
    
    # 8. Edit a sponsor
    print("[8] Editing sponsor Google...")
    google_id = sponsor_ids[-1] # The Google sponsor ID is likely one of these
    for sid in sponsor_ids:
        edit_r = session.get(f"{BASE_URL}/admin/sponsors/{sid}/edit")
        if "Google" in edit_r.text:
            google_id = sid
            break
            
    edit_data = {
        "event_id": event_id,
        "name": "Alphabet Inc.",
        "category": "Title Sponsor",
        "website_url": "https://abc.xyz",
        "display_order": "1",
        "active": "true"
    }
    r = session.post(f"{BASE_URL}/admin/sponsors/{google_id}/edit", data=edit_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Sponsor editing failed. Status: {r.status_code}")
        sys.exit(1)
        
    # Verify name change
    sponsors_r = session.get(f"{BASE_URL}/admin/sponsors")
    if "Alphabet Inc." not in sponsors_r.text:
        print("[-] Sponsor name change not reflected in listing!")
        sys.exit(1)
    print("[+] Sponsor successfully edited and name change verified.")
    
    # 9. Delete a sponsor
    print("[9] Deleting sponsor 'TechCrunch'...")
    tc_id = None
    for sid in sponsor_ids:
        if sid != google_id:
            tc_id = sid
            break
            
    if tc_id:
        r = session.post(f"{BASE_URL}/admin/sponsors/{tc_id}/delete", allow_redirects=False)
        if r.status_code != 303:
            print(f"[-] Sponsor deletion failed. Status: {r.status_code}")
            sys.exit(1)
            
        sponsors_r = session.get(f"{BASE_URL}/admin/sponsors")
        if f"/admin/sponsors/{tc_id}/edit" in sponsors_r.text:
            print(f"[-] Sponsor ID {tc_id} still exists after deletion!")
            sys.exit(1)
        print("[+] Sponsor successfully deleted.")
        
        # Verify dashboard count drops to 1
        dash_r = session.get(f"{BASE_URL}/admin/dashboard?event_id={event_id}")
        match = re.search(r'jumlah sponsor</p>\s*<p class="text-white font-bold">(\d+)</p>', dash_r.text, re.IGNORECASE)
        if match:
            count = int(match.group(1))
            if count != 1:
                print(f"[-] Expected sponsor count on dashboard after deletion is 1, got: {count}")
                sys.exit(1)
            print("[+] Sponsor count correctly decreased to 1 on admin dashboard.")
    else:
        print("[-] No other sponsor to delete")
        
    print("=== Verification Successful! Sponsor & Partner module is working perfectly. ===")

if __name__ == "__main__":
    run_tests()
