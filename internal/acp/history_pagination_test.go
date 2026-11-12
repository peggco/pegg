package acp

import (
	"testing"
	"time"

	"github.com/peggco/pegg/internal/llm"
	"github.com/peggco/pegg/internal/session"
)

func TestHistoryPage(t *testing.T) {
	srv, mgr, _ := newHistoryServer(t)
	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := mgr.AppendMessage(sess.ID, llm.UserMessage("hi")); err != nil {
			t.Fatal(err)
		}
	}

	cases := []struct {
		name    string
		before  int
		limit   int
		want    []int
		oldest  int
		hasMore bool
	}{
		{name: "newest page", before: 0, limit: 2, want: []int{4, 5}, oldest: 4, hasMore: true},
		{name: "middle page", before: 4, limit: 2, want: []int{2, 3}, oldest: 2, hasMore: true},
		{name: "oldest page", before: 2, limit: 2, want: []int{1}, oldest: 1, hasMore: false},
		{name: "exhausted", before: 1, limit: 2, want: nil, oldest: 0, hasMore: false},
		{name: "all without limit", before: 0, limit: 0, want: []int{1, 2, 3, 4, 5}, oldest: 1, hasMore: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, oldest, hasMore, err := srv.historyPage(sess.ID, tc.before, tc.limit)
			if err != nil {
				t.Fatal(err)
			}
			if hasMore != tc.hasMore || oldest != tc.oldest {
				t.Fatalf("oldest = %d hasMore = %v, want %d/%v", oldest, hasMore, tc.oldest, tc.hasMore)
			}
			if len(page) != len(tc.want) {
				t.Fatalf("page = %+v, want seqs %v", page, tc.want)
			}
			for i, wantSeq := range tc.want {
				if page[i].Seq != wantSeq {
					t.Fatalf("page[%d].Seq = %d, want %d (page %+v)", i, page[i].Seq, wantSeq, page)
				}
			}
		})
	}
}

func TestHistoryPageBoundsBeforeActiveRunBaseSeq(t *testing.T) {
	srv, mgr, _ := newHistoryServer(t)
	sess, err := mgr.Create(session.CreateOptions{Provider: "p", Model: "m"})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 5; i++ {
		if _, err := mgr.AppendMessage(sess.ID, llm.UserMessage("hi")); err != nil {
			t.Fatal(err)
		}
	}

	rs, err := mgr.Runs()
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	run := session.Run{
		ID: "run-1", SessionID: sess.ID, State: session.RunStateRunning,
		OwnerInstance: "other", AgentID: "remote-agent", BaseSeq: 3,
		StartedAt: now, UpdatedAt: now,
	}
	if err := rs.CreateRun(run); err != nil {
		t.Fatal(err)
	}

	page, oldest, hasMore, err := srv.historyPage(sess.ID, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 3 || page[0].Seq != 1 || page[2].Seq != 3 || oldest != 1 || hasMore {
		t.Fatalf("base seq page = %+v oldest=%d more=%v", page, oldest, hasMore)
	}

	page, oldest, hasMore, err = srv.historyPage(sess.ID, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 2 || page[0].Seq != 2 || page[1].Seq != 3 || oldest != 2 || !hasMore {
		t.Fatalf("bounded page = %+v oldest=%d more=%v", page, oldest, hasMore)
	}

	page, oldest, hasMore, err = srv.historyPage(sess.ID, 10, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(page) != 3 || page[0].Seq != 1 || page[2].Seq != 3 || oldest != 1 || hasMore {
		t.Fatalf("before clamped to base seq = %+v oldest=%d more=%v", page, oldest, hasMore)
	}
}
