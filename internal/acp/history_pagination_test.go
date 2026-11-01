package acp

import (
	"testing"

	"github.com/peggco/pegg/internal/session"
)

func TestPaginateHistory(t *testing.T) {
	msgs := []session.Message{{Seq: 1}, {Seq: 2}, {Seq: 3}, {Seq: 4}, {Seq: 5}}

	page, oldest, more := paginateHistory(msgs, 0, 2)
	if len(page) != 2 || page[0].Seq != 4 || page[1].Seq != 5 || oldest != 4 || !more {
		t.Fatalf("latest page = %+v oldest=%d more=%v", page, oldest, more)
	}

	page, oldest, more = paginateHistory(msgs, 4, 2)
	if len(page) != 2 || page[0].Seq != 2 || page[1].Seq != 3 || oldest != 2 || !more {
		t.Fatalf("middle page = %+v oldest=%d more=%v", page, oldest, more)
	}

	page, oldest, more = paginateHistory(msgs, 2, 2)
	if len(page) != 1 || page[0].Seq != 1 || oldest != 1 || more {
		t.Fatalf("last page = %+v oldest=%d more=%v", page, oldest, more)
	}

	page, oldest, more = paginateHistory(msgs, 1, 2)
	if len(page) != 0 || oldest != 0 || more {
		t.Fatalf("exhausted page = %+v oldest=%d more=%v", page, oldest, more)
	}
}
