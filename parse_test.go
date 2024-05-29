/*
 * Copyright 2024 Daniel C. Brotsky. All rights reserved.
 * All the copyrighted work in this repository is licensed under the
 * GNU Affero General Public License v3, reproduced in the LICENSE file.
 */

package tracker

import (
	"fmt"
	"os"
	"testing"
)

func TestParseSingleSessionLogs(t *testing.T) {
	for i := 1; i <= 2; i++ {
		path := fmt.Sprintf("testdata/indesign-single-session-%d.txt", i)
		buffer, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("Failed to read file %s: %s", path, err)
		}
		sessions := parseLog(string(buffer))
		if len(sessions) != 1 {
			t.Fatalf("Expected 1 session, got %d", len(sessions))
		}
		session := sessions[0]
		if session.appId != "InDesign1" {
			t.Errorf("%d: Expected appId %q, got %q", i, "InDesign1", sessions[0].appId)
		}
		if session.appVersion != "19.2" {
			t.Errorf("%d: Expected appVersion %q, got %q", i, "19.2", session.appVersion)
		}
		if session.osName != "MAC" {
			t.Errorf("%d: Expected osName %q, got %q", i, "MAC", session.osName)
		}
		if session.osVersion != "14.3.1" {
			t.Errorf("%d: Expected osVersion %q, got %q", i, "14.3.1", session.osVersion)
		}
		if session.nglVersion != "1.35.0.19" {
			t.Errorf("%d: Expected nglVersion %q, got %q", i, "1.35.0.19", session.nglVersion)
		}
		if session.appLocale != "en_US" {
			t.Errorf("%d: Expected appLocale %q, got %q", i, "en_US", session.appLocale)
		}
		if session.userId != "9f22a90139cbb9f1676b0113e1fb574976dc550a" {
			t.Errorf("%d: Expected userId %q, got %q", i, "9f22a90139cbb9f1676b0113e1fb574976dc550a", session.userId)
		}
	}
}

func TestParseSplitSessionLogs(t *testing.T) {
	var buffer []byte
	var err error
	var sessions []logSession
	path1 := fmt.Sprintf("testdata/indesign-split-session-1-1.txt")
	path2 := fmt.Sprintf("testdata/indesign-split-session-1-2.txt")
	buffer, err = os.ReadFile(path1)
	if err != nil {
		t.Fatalf("Failed to read file %s: %s", path1, err)
	}
	sessions = parseLog(string(buffer))
	if len(sessions) != 1 {
		t.Fatalf("%s: Expected 1 session, got %d", path1, len(sessions))
	}
	session1 := sessions[0]
	buffer, err = os.ReadFile(path2)
	if err != nil {
		t.Fatalf("Failed to read file %s: %s", path2, err)
	}
	sessions = parseLog(string(buffer))
	if len(sessions) != 1 {
		t.Fatalf("%s: Expected 1 session, got %d", path2, len(sessions))
	}
	session2 := sessions[0]
	if session1.sessionId != session2.sessionId {
		t.Errorf("Session ids differ in split-session logs")
	}
	if session1.launchDuration >= session2.launchDuration {
		t.Errorf(
			"Session 2 launch duration (%v) < Session 1 launch duration (%v)",
			session2.launchDuration, session1.launchDuration,
		)
	}
}

func TestParseMultiSessionLogs(t *testing.T) {
	var buffer []byte
	var err error
	var sessions []logSession
	path1 := fmt.Sprintf("testdata/indesign-multi-session-1-1.txt")
	path2 := fmt.Sprintf("testdata/indesign-multi-session-1-2.txt")
	buffer, err = os.ReadFile(path1)
	if err != nil {
		t.Fatalf("Failed to read file %s: %s", path1, err)
	}
	sessions = parseLog(string(buffer))
	if len(sessions) != 1 {
		t.Fatalf("%s: Expected 1 session, got %d", path1, len(sessions))
	}
	session1 := sessions[0]
	buffer, err = os.ReadFile(path2)
	if err != nil {
		t.Fatalf("Failed to read file %s: %s", path2, err)
	}
	sessions = parseLog(string(buffer))
	if len(sessions) != 2 {
		t.Fatalf("%s: Expected 2 sessions, got %d", path2, len(sessions))
	}
	session2 := sessions[0]
	session3 := sessions[1]
	if session1.sessionId != session2.sessionId {
		t.Errorf("Session ids differ in split-multi-session logs")
	}
	if session2.sessionId == session3.sessionId {
		t.Errorf("Session ids don't differ in multi-session logs")
	}
	if session1.launchTime.Compare(session3.launchTime) != -1 {
		t.Errorf(
			"Session 1 launch time (%v) < Session 3 launch time (%v)",
			session1.launchTime, session3.launchTime,
		)
	}
}