package aggregator

import (
	"context"
	"encoding/json"
	"errors"

	"golang.org/x/sync/errgroup"
)

// Aggregate fetches every id with at most cfg.MaxConcurrency calls in flight.
func (s *Service) Aggregate(ctx context.Context, ids []int) Response {
	// Every goroutine writes only to its own index, so no mutex is needed.
	data := make([]json.RawMessage, len(ids))
	errs := make([]error, len(ids))

	var g errgroup.Group
	g.SetLimit(s.cfg.MaxConcurrency)
	for i, id := range ids {
		g.Go(func() error {
			data[i], errs[i] = s.fetch(ctx, id)
			return nil // failures are data here, not a reason to stop the others
		})
	}
	_ = g.Wait()

	resp := Response{Results: make([]Result, 0, len(ids)), Errors: []Failure{}}
	for i, id := range ids {
		if errs[i] != nil {
			resp.Errors = append(resp.Errors, Failure{ID: id, Error: describe(errs[i])})
			continue
		}
		resp.Results = append(resp.Results, Result{ID: id, Data: data[i]})
	}
	return resp
}

func describe(err error) string {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return "cancelled"
	}
	return err.Error()
}
