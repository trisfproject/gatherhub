#!/usr/bin/env python3
import requests
import re
import sys
import time

BASE_URL = "http://localhost:3000"

def run_tests():
    print("=== Starting Event Task Management Verification ===")
    
    session = requests.Session()
    
    # 1. Login as admin
    print("[1] Logging in as admin...")
    login_data = {
        "username": "trisf",
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
    
    # 2. Create a test event
    test_slug = f"task-test-{int(time.time())}"
    print(f"[2] Creating test event with slug '{test_slug}'...")
    event_data = {
        "title": "Task Test Event",
        "slug": test_slug,
        "description": "Event for testing task preparation functionality",
        "event_date": "2026-08-20",
        "event_time": "09:00 - 12:00",
        "location": "JCC Senayan, Jakarta",
        "price": "10000",
        "admin_name": "Test Task Admin",
        "admin_whatsapp": "08121111111",
        "max_participants": "50",
        "status": "PUBLISHED"
    }
    
    r = session.post(f"{BASE_URL}/admin/events/create", data=event_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create event. Status: {r.status_code}")
        sys.exit(1)
            
    print(f"[+] Event created successfully with slug: {test_slug}")
    
    # Find Event ID
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
    
    # 3. Create 3 Tasks
    print("[3] Creating 3 tasks...")
    
    # Task 1: Open, High priority, due in future
    task1_data = {
        "event_id": event_id,
        "title": "Registration setup",
        "description": "Set up registration forms and ticket prices",
        "category": "Registration",
        "priority": "High",
        "due_date": "2026-08-01",
        "assigned_to": "Alice",
        "status": "Todo"
    }
    r = session.post(f"{BASE_URL}/admin/tasks/create", data=task1_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Task 1. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Task 1 (Todo) created.")

    # Task 2: Open, Critical priority, overdue (due in past)
    task2_data = {
        "event_id": event_id,
        "title": "Logistics booking",
        "description": "Book truck for transporting sound system",
        "category": "Logistics",
        "priority": "Critical",
        "due_date": "2026-05-01",
        "assigned_to": "Bob",
        "status": "In Progress"
    }
    r = session.post(f"{BASE_URL}/admin/tasks/create", data=task2_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Task 2. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Task 2 (In Progress, Overdue) created.")

    # Task 3: Completed, Low priority, due in future
    task3_data = {
        "event_id": event_id,
        "title": "Sponsor brochure",
        "description": "Design sponsor proposal and brochure",
        "category": "Sponsor",
        "priority": "Low",
        "due_date": "2026-07-15",
        "assigned_to": "Charlie",
        "status": "Done"
    }
    r = session.post(f"{BASE_URL}/admin/tasks/create", data=task3_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to create Task 3. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Task 3 (Done) created.")

    # 4. Verify tasks list page
    print("[4] Verifying tasks list page...")
    tasks_r = session.get(f"{BASE_URL}/admin/tasks?event_id={event_id}")
    if tasks_r.status_code != 200:
        print(f"[-] Tasks list page failed. Status: {tasks_r.status_code}")
        sys.exit(1)

    if "Registration setup" not in tasks_r.text or "Logistics booking" not in tasks_r.text or "Sponsor brochure" not in tasks_r.text:
        print("[-] Registered tasks are missing from tasks list page!")
        sys.exit(1)
    print("[+] All tasks rendered on list page.")

    # Find task IDs
    task_ids = re.findall(r'href="/admin/tasks/(\d+)/edit"', tasks_r.text)
    print(f"[+] Found task edit links: {task_ids}")
    if len(task_ids) < 3:
        print("[-] Missing task IDs")
        sys.exit(1)

    # 5. Verify dashboard task stats
    print("[5] Verifying task stats on admin dashboard...")
    dash_r = session.get(f"{BASE_URL}/admin/dashboard?event_id={event_id}")
    if dash_r.status_code != 200:
        print(f"[-] Dashboard fetch failed. Status: {dash_r.status_code}")
        sys.exit(1)

    # Assert open tasks count = 2
    open_match = re.search(r'id="dash-open-tasks">(\d+)</span>', dash_r.text)
    overdue_match = re.search(r'id="dash-overdue-tasks">(\d+)</span>', dash_r.text)
    completed_match = re.search(r'id="dash-completed-tasks">(\d+)</span>', dash_r.text)

    if not open_match or not overdue_match or not completed_match:
        print("[-] Task stats not found on admin dashboard!")
        sys.exit(1)

    open_cnt = int(open_match.group(1))
    overdue_cnt = int(overdue_match.group(1))
    completed_cnt = int(completed_match.group(1))

    print(f"[+] Dashboard stats: Open: {open_cnt}, Overdue: {overdue_cnt}, Completed: {completed_cnt}")
    if open_cnt != 2 or overdue_cnt != 1 or completed_cnt != 1:
        print("[-] Initial task stats assertions failed!")
        sys.exit(1)
    print("[+] Initial dashboard task stats verified.")

    # 6. Edit Task 1 to Done status
    # Let's locate task1 ID. We can request task list and parse the edit page text to map IDs
    task1_id = None
    task3_id = None
    for tid in task_ids:
        edit_r = session.get(f"{BASE_URL}/admin/tasks/{tid}/edit")
        if "Registration setup" in edit_r.text:
            task1_id = tid
        elif "Sponsor brochure" in edit_r.text:
            task3_id = tid

    if not task1_id:
        print("[-] Could not identify Task 1 ID")
        sys.exit(1)

    print(f"[6] Editing Task 1 (ID: {task1_id}) status to Done...")
    edit_task_data = {
        "event_id": event_id,
        "title": "Registration setup",
        "description": "Set up registration forms and ticket prices",
        "category": "Registration",
        "priority": "High",
        "due_date": "2026-08-01",
        "assigned_to": "Alice",
        "status": "Done" # changed
    }
    r = session.post(f"{BASE_URL}/admin/tasks/{task1_id}/edit", data=edit_task_data, allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to edit Task 1. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Task 1 successfully updated to Done.")

    # Verify updated stats on dashboard (Open = 1, Overdue = 1, Completed = 2)
    dash_r = session.get(f"{BASE_URL}/admin/dashboard?event_id={event_id}")
    open_match = re.search(r'id="dash-open-tasks">(\d+)</span>', dash_r.text)
    overdue_match = re.search(r'id="dash-overdue-tasks">(\d+)</span>', dash_r.text)
    completed_match = re.search(r'id="dash-completed-tasks">(\d+)</span>', dash_r.text)

    open_cnt = int(open_match.group(1))
    overdue_cnt = int(overdue_match.group(1))
    completed_cnt = int(completed_match.group(1))

    print(f"[+] Updated Dashboard stats: Open: {open_cnt}, Overdue: {overdue_cnt}, Completed: {completed_cnt}")
    if open_cnt != 1 or overdue_cnt != 1 or completed_cnt != 2:
        print("[-] Task stats assertions after update failed!")
        sys.exit(1)
    print("[+] Updated dashboard task stats verified.")

    # 7. Delete Task 3
    if not task3_id:
        print("[-] Could not identify Task 3 ID")
        sys.exit(1)

    print(f"[7] Deleting Task 3 (ID: {task3_id})...")
    r = session.post(f"{BASE_URL}/admin/tasks/{task3_id}/delete", allow_redirects=False)
    if r.status_code != 303:
        print(f"[-] Failed to delete Task 3. Status: {r.status_code}")
        sys.exit(1)
    print("[+] Task 3 deleted.")

    # Verify updated stats on dashboard (Open = 1, Overdue = 1, Completed = 1)
    dash_r = session.get(f"{BASE_URL}/admin/dashboard?event_id={event_id}")
    open_match = re.search(r'id="dash-open-tasks">(\d+)</span>', dash_r.text)
    overdue_match = re.search(r'id="dash-overdue-tasks">(\d+)</span>', dash_r.text)
    completed_match = re.search(r'id="dash-completed-tasks">(\d+)</span>', dash_r.text)

    open_cnt = int(open_match.group(1))
    overdue_cnt = int(overdue_match.group(1))
    completed_cnt = int(completed_match.group(1))

    print(f"[+] Final Dashboard stats: Open: {open_cnt}, Overdue: {overdue_cnt}, Completed: {completed_cnt}")
    if open_cnt != 1 or overdue_cnt != 1 or completed_cnt != 1:
        print("[-] Task stats assertions after deletion failed!")
        sys.exit(1)
    print("[+] Final dashboard task stats verified.")

    print("=== Verification Successful! Event Task Management module is working perfectly. ===")

if __name__ == "__main__":
    run_tests()
