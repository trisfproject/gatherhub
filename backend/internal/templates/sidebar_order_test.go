package templates_test

import (
	"regexp"
	"strings"
	"testing"

	"github.com/trisfproject/gatherhub/internal/handlers"
	templ "github.com/trisfproject/gatherhub/internal/templates"
	"io/fs"
)

// canonicalMenuOrder is the single source of truth for sidebar menu order.
// Any deviation in a template file is a bug.
var canonicalMenuOrder = []string{
	"/admin/dashboard",
	"/admin/participants",
	"/admin/events",
	"/admin/notifications",
	"/admin/audit-logs",
	"/admin/backups",
	"/admin/checkin",
	"/admin/attendance",
	"/admin/broadcasts",
	"/admin/transportation",
	"/admin/system",
	// /admin/admins and /admin/settings are SUPER_ADMIN only (conditional)
	// /  is "Halaman Publik" — external link
}

// hrefRE matches href="/admin/... inside nav-item anchor tags.
var hrefRE = regexp.MustCompile(`href="(/admin/[^"]+)"`)

// TestSidebarMenuOrderIsIdentical verifies that every admin template
// either:
//   (a) uses {{template "admin_sidebar" .}} (correct: single source of truth)
//   (b) does NOT contain hard-coded nav-item links at all (e.g. login page)
//
// If a template contains hard-coded nav links, we verify they are in canonical
// order.
func TestSidebarMenuOrderIsIdentical(t *testing.T) {
	t.Helper()

	// Read all admin_*.html template files from the embedded FS
	entries, err := fs.ReadDir(templ.Files, ".")
	if err != nil {
		t.Fatalf("failed to read embedded template dir: %v", err)
	}

	// First verify the sidebar partial exists
	sidebarExists := false
	for _, e := range entries {
		if e.Name() == "admin_sidebar.html" {
			sidebarExists = true
			break
		}
	}
	if !sidebarExists {
		t.Fatal("admin_sidebar.html does not exist — canonical sidebar partial is missing")
	}

	for _, e := range entries {
		name := e.Name()
		if !strings.HasPrefix(name, "admin_") || !strings.HasSuffix(name, ".html") {
			continue
		}
		if name == "admin_sidebar.html" || name == "admin_login.html" {
			continue // these are either the canonical source or have no sidebar
		}

		data, err := templ.Files.ReadFile(name)
		if err != nil {
			t.Errorf("[%s] failed to read: %v", name, err)
			continue
		}
		content := string(data)

		// Check: template should call the sidebar partial
		if strings.Contains(content, `{{template "admin_sidebar" .}}`) {
			// Correct — no further checks needed
			continue
		}

		// If it doesn't use the partial but has nav-items, check order
		links := hrefRE.FindAllStringSubmatch(content, -1)
		if len(links) == 0 {
			// No nav links at all — fine (e.g. purely content pages)
			continue
		}

		// Extract just the href values
		extracted := make([]string, 0, len(links))
		seen := map[string]bool{}
		for _, l := range links {
			href := l[1]
			// Only consider the canonical admin paths; skip sub-paths like /admin/participants/1
			for _, canonical := range canonicalMenuOrder {
				if href == canonical && !seen[href] {
					extracted = append(extracted, href)
					seen[href] = true
					break
				}
			}
		}

		if len(extracted) == 0 {
			continue
		}

		// Verify order matches canonical
		canIdx := 0
		for _, link := range extracted {
			for canIdx < len(canonicalMenuOrder) && canonicalMenuOrder[canIdx] != link {
				canIdx++
			}
			if canIdx >= len(canonicalMenuOrder) {
				t.Errorf("[%s] nav link %q appears after its canonical position or is out of order", name, link)
				break
			}
			canIdx++
		}

		// Also check if this template is missing the sidebar partial call — warn
		t.Errorf("[%s] MISSING {{template \"admin_sidebar\" .}} — should use the canonical partial instead of hard-coded sidebar HTML", name)
	}
}

// TestSidebarPartialContainsAllMenuItems verifies BuildSidebarItems
// contains all canonical menu links in the correct order.
func TestSidebarPartialContainsAllMenuItems(t *testing.T) {
	items := handlers.BuildSidebarItems("SUPER_ADMIN", "", nil)

	found := make([]string, 0)
	for _, item := range items {
		// Only check items that start with /admin/
		if strings.HasPrefix(item.URL, "/admin/") {
			found = append(found, item.URL)
		}
	}

	// Check each canonical item is present
	for i, canonical := range canonicalMenuOrder {
		if i >= len(found) {
			t.Errorf("BuildSidebarItems is missing menu item: %q", canonical)
			continue
		}
		if found[i] != canonical {
			t.Errorf("BuildSidebarItems menu order wrong at position %d: got %q, want %q", i, found[i], canonical)
		}
	}
}
