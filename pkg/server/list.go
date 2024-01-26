package server

import (
	"bytes"
	"context"
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

func (l *LimitedServer) list(ctx context.Context, r *etcdserverpb.RangeRequest) (*RangeResponse, error) {
	if len(r.RangeEnd) == 0 {
		return nil, fmt.Errorf("invalid range end length of 0")
	}

	prefix := string(append(r.RangeEnd[:len(r.RangeEnd)-1], r.RangeEnd[len(r.RangeEnd)-1]-1))
	if !strings.HasSuffix(prefix, "/") {
		prefix = prefix + "/"
	}
	start := string(bytes.TrimRight(r.Key, "\x00"))
	revision := r.Revision

	if r.CountOnly {
		rev, count, err := l.backend.Count(ctx, prefix, revision)
		if err != nil {
			return nil, err
		}
		logrus.Tracef("LIST COUNT key=%s, end=%s, revision=%d, currentRev=%d count=%d", r.Key, r.RangeEnd, revision, rev, count)
		return &RangeResponse{
			Header: txnHeader(rev),
			Count:  count,
		}, nil
	}

	limit := r.Limit
	if limit > 0 {
		limit++
	}

	rev, kvs, err := l.backend.List(ctx, prefix, start, limit, revision)
	if err != nil {
		return nil, err
	}

	logrus.Tracef("LIST key=%s, end=%s, revision=%d, currentRev=%d count=%d, limit=%d", r.Key, r.RangeEnd, revision, rev, len(kvs), r.Limit)
	resp := &RangeResponse{
		Header: txnHeader(rev),
		Count:  int64(len(kvs)),
		Kvs:    kvs,
	}

	// count the actual number of results if there are more items in the db.
	if limit > 0 && resp.Count > r.Limit {
		resp.More = true
		resp.Kvs = kvs[0 : limit-1]

		if revision == 0 {
			revision = rev
		}

		rev, resp.Count, err = l.backend.Count(ctx, prefix, revision)
		if err != nil {
			return nil, err
		}
		logrus.Tracef("LIST COUNT key=%s, end=%s, revision=%d, currentRev=%d count=%d", r.Key, r.RangeEnd, revision, rev, resp.Count)
		resp.Header = txnHeader(rev)
	}

	return resp, nil
}
