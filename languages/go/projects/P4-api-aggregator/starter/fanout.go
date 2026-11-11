package aggregator

import "context"

// Aggregate fetches every id with at most cfg.MaxConcurrency calls in flight.
//
// TODO:
//   - run the fetches concurrently (golang.org/x/sync/errgroup with SetLimit
//     is the idiomatic tool; each goroutine writes only to its own slot);
//   - put successes in Results and failures in Errors, both in the order of
//     ids; both slices must be non-nil so they encode as [] and not null;
//   - when ctx is cancelled or its deadline passes, report the unfinished ids
//     as failures with the message "cancelled".
func (s *Service) Aggregate(ctx context.Context, ids []int) Response {
	return Response{}
}
